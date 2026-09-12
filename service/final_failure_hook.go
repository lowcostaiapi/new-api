package service

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// 最终失败实时上报：一次请求重试耗尽（或压根不允许重试）、错误即将回到用户手里的那一刻，
// 把这次失败异步推给监控看板，让 Telegram 播报能在秒级到达。
//
// 看板原本只能反扫 new-api 的 logs 表，且必须等 5 分钟才敢断定「这次是最终失败」——
// 否则会把「重试到别的渠道后跑了很久才成功」的长流式请求误报成故障。这个钩子直接在
// 失败发生的位置上报，不需要猜，也顺带覆盖了「分组下一个可用渠道都没有」这种连日志
// 都不写的场景。
//
// 未配置 FINAL_FAILURE_WEBHOOK_URL 时，下面的逻辑在第一行就返回，行为与官方版一致。
const (
	finalFailureSourceRelay    = "relay"    // 走完重试循环后仍然失败
	finalFailureSourceDispatch = "dispatch" // 分发阶段就没有可用渠道，请求根本没发出去
	finalFailureMessageLimit   = 500
)

type finalFailureHookConfig struct {
	url        string
	token      string
	timeout    time.Duration
	rateLimit  int
	skipStatus map[int]bool
}

var (
	finalFailureConfigOnce sync.Once
	finalFailureHook       finalFailureHookConfig

	finalFailureRate struct {
		sync.Mutex
		windowSec int64
		sent      int
		dropped   int
	}
)

func finalFailureConfig() finalFailureHookConfig {
	finalFailureConfigOnce.Do(func() {
		finalFailureHook = finalFailureHookConfig{
			url:       strings.TrimSpace(common.GetEnvOrDefaultString("FINAL_FAILURE_WEBHOOK_URL", "")),
			token:     common.GetEnvOrDefaultString("FINAL_FAILURE_WEBHOOK_TOKEN", ""),
			timeout:   time.Duration(common.GetEnvOrDefault("FINAL_FAILURE_WEBHOOK_TIMEOUT_MS", 3000)) * time.Millisecond,
			rateLimit: common.GetEnvOrDefault("FINAL_FAILURE_WEBHOOK_RATE_LIMIT", 30),
			// 只按结构化 skip_retry 默认排除客户端错误；状态码静默必须由运维显式配置。
			skipStatus: map[int]bool{},
		}
		for _, s := range strings.Split(common.GetEnvOrDefaultString("FINAL_FAILURE_SKIP_STATUS", ""), ",") {
			if code, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				finalFailureHook.skipStatus[code] = true
			}
		}
		if finalFailureHook.url != "" {
			logger.LogInfo(nil, "最终失败实时上报已启用："+finalFailureHook.url)
		}
	})
	return finalFailureHook
}

type finalFailureReport struct {
	Ts          int64    `json:"ts"`
	Source      string   `json:"source"`
	RequestId   string   `json:"request_id"`
	UserId      int      `json:"user_id"`
	Username    string   `json:"username"`
	Group       string   `json:"group"`
	Model       string   `json:"model"`
	StatusCode  int      `json:"status_code"`
	ErrorCode   string   `json:"error_code"`
	ErrorType   string   `json:"error_type"`
	Message     string   `json:"message"`
	ChannelId   int      `json:"channel_id"`
	ChannelName string   `json:"channel_name"`
	UseChannel  []string `json:"use_channel"`
	RetryIndex  int      `json:"retry_index"`
	Path        string   `json:"path"`
}

// NotifyFinalFailure 在中继重试循环结束、错误确定要返回给用户时调用。
func NotifyFinalFailure(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	cfg := finalFailureConfig()
	if cfg.url == "" || c == nil || info == nil || apiErr == nil {
		return
	}
	if types.IsSkipRetryError(apiErr) || cfg.skipStatus[apiErr.StatusCode] {
		return
	}

	message := apiErr.Error()
	if len(message) > finalFailureMessageLimit {
		message = message[:finalFailureMessageLimit] + "…"
	}
	group := info.UsingGroup
	if group == "" {
		group = info.UserGroup
	}
	sendFinalFailureReport(cfg, &finalFailureReport{
		Ts:          time.Now().UnixMilli(),
		Source:      finalFailureSourceRelay,
		RequestId:   info.RequestId,
		UserId:      info.UserId,
		Username:    common.GetContextKeyString(c, constant.ContextKeyUserName),
		Group:       group,
		Model:       info.OriginModelName,
		StatusCode:  apiErr.StatusCode,
		ErrorCode:   string(apiErr.GetErrorCode()),
		ErrorType:   string(apiErr.GetErrorType()),
		Message:     message,
		ChannelId:   common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		ChannelName: common.GetContextKeyString(c, constant.ContextKeyChannelName),
		UseChannel:  c.GetStringSlice("use_channel"),
		RetryIndex:  info.RetryIndex,
		Path:        c.Request.URL.Path,
	})
}

// NotifyDispatchFinalFailure 在分发阶段就选不出渠道时调用：请求没有进入中继，
// 不会写错误日志，看板反扫日志的那条路永远看不见这类故障。
func NotifyDispatchFinalFailure(c *gin.Context, group string, modelName string, statusCode int, message string) {
	cfg := finalFailureConfig()
	if cfg.url == "" || c == nil {
		return
	}
	if cfg.skipStatus[statusCode] {
		return
	}
	if len(message) > finalFailureMessageLimit {
		message = message[:finalFailureMessageLimit] + "…"
	}
	sendFinalFailureReport(cfg, &finalFailureReport{
		Ts:         time.Now().UnixMilli(),
		Source:     finalFailureSourceDispatch,
		RequestId:  common.GetContextKeyString(c, common.RequestIdKey),
		UserId:     common.GetContextKeyInt(c, constant.ContextKeyUserId),
		Username:   common.GetContextKeyString(c, constant.ContextKeyUserName),
		Group:      group,
		Model:      modelName,
		StatusCode: statusCode,
		ErrorCode:  string(types.ErrorCodeModelNotFound),
		Message:    message,
		Path:       c.Request.URL.Path,
	})
}

func sendFinalFailureReport(cfg finalFailureHookConfig, report *finalFailureReport) {
	// 上游大面积挂掉时失败会成簇涌来，这里按秒限流，丢弃的条数留在日志里可查。
	// 看板侧本来就按「分组×模型」聚合播报，丢几条不改变结论。
	nowSec := report.Ts / 1000
	finalFailureRate.Lock()
	if finalFailureRate.windowSec != nowSec {
		if finalFailureRate.dropped > 0 {
			logger.LogWarn(nil, "最终失败上报限流，丢弃 "+strconv.Itoa(finalFailureRate.dropped)+" 条")
		}
		finalFailureRate.windowSec = nowSec
		finalFailureRate.sent = 0
		finalFailureRate.dropped = 0
	}
	if finalFailureRate.sent >= cfg.rateLimit {
		finalFailureRate.dropped++
		finalFailureRate.Unlock()
		return
	}
	finalFailureRate.sent++
	finalFailureRate.Unlock()

	body, err := common.Marshal(report)
	if err != nil {
		logger.LogError(nil, "最终失败上报序列化失败: "+err.Error())
		return
	}
	gopool.Go(func() {
		req, err := http.NewRequest(http.MethodPost, cfg.url, bytes.NewReader(body))
		if err != nil {
			logger.LogError(nil, "最终失败上报构造请求失败: "+err.Error())
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.token != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.token)
		}
		client := &http.Client{Timeout: cfg.timeout}
		resp, err := client.Do(req)
		if err != nil {
			logger.LogError(nil, "最终失败上报发送失败: "+err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			logger.LogError(nil, "最终失败上报被拒绝: HTTP "+strconv.Itoa(resp.StatusCode))
		}
	})
}
