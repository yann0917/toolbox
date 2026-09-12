package volcengine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/yann0917/toolbox/internal/provider/volcengine/sauc"
)

const (
	// asrAUCBaseURL 大模型录音文件识别（异步 submit/query）REST 服务地址。
	asrAUCBaseURL = "https://openspeech.bytedance.com"
	// asrAUCSubmitPath 提交识别任务路径。
	asrAUCSubmitPath = "/api/v3/auc/bigmodel/submit"
	// asrAUCQueryPath 查询识别结果路径。
	asrAUCQueryPath = "/api/v3/auc/bigmodel/query"
	// asrAUCResourceID 异步录音文件识别资源 ID（官方文档 6561/1354868）。
	asrAUCResourceID = "volc.seedasr.auc"
	// asrAUCCodeOK 成功状态码（响应头 X-Api-Status-Code）。
	asrAUCCodeOK = "20000000"
)

// ASRAUCClient 火山引擎大模型录音文件识别（异步 submit/query）REST 客户端。
// 鉴权同 WS 客户端：SpeechCred（新版 X-Api-Key 或老版 X-Api-App-Key + X-Api-Access-Key）。
// 用法：Submit 提交音频 URL 得到任务 ID，轮询 Query 直到 status 为 Completed/Failed。
type ASRAUCClient struct {
	resty *resty.Client
	cred  SpeechCred
}

func NewASRAUCClient(cred SpeechCred) *ASRAUCClient {
	return NewASRAUCClientWithBaseURL(cred, asrAUCBaseURL)
}

// NewASRAUCClientWithBaseURL 供测试注入 mock 地址。
func NewASRAUCClientWithBaseURL(cred SpeechCred, baseURL string) *ASRAUCClient {
	r := resty.New().SetBaseURL(baseURL).
		SetHeader("Content-Type", "application/json").
		SetTimeout(60 * time.Second)
	return &ASRAUCClient{resty: r, cred: cred}
}

// asrAUCQueryResp 查询任务响应（与 WS 响应同构；utterances 缺失时 Segments 为空，上层跳过 SRT）。
type asrAUCQueryResp struct {
	ID     string `json:"id"`
	Status string `json:"status"` // Queuing|Running|Completed|Failed
	Result struct {
		Text       string `json:"text"`
		Utterances []struct {
			Text      string `json:"text"`
			StartTime int64  `json:"start_time"`
			EndTime   int64  `json:"end_time"`
		} `json:"utterances"`
	} `json:"result"`
	AudioInfo struct {
		Duration int64 `json:"duration"`
	} `json:"audio_info"`
}

// Submit 提交录音文件识别任务：POST submit，body {"audio_url":...}，任务 ID 即客户端生成的
// X-Api-Request-Id（UUID）。成功判定：响应头 X-Api-Status-Code == 20000000。
func (c *ASRAUCClient) Submit(ctx context.Context, audioURL string) (string, error) {
	headers := sauc.NewAuthHeaderFrom(c.cred.APIKey, c.cred.AppID, c.cred.AccessToken, asrAUCResourceID)
	taskID := headers.Get("X-Api-Request-Id")
	headers.Set("X-Api-Sequence", "-1")

	httpResp, err := c.resty.R().
		SetContext(ctx).
		SetHeaders(aucHeaderMap(headers)).
		SetBody(map[string]string{"audio_url": audioURL}).
		Post(asrAUCSubmitPath)
	if err != nil {
		return "", fmt.Errorf("提交火山 ASR 识别任务失败: %w", err)
	}
	if err := checkAUCStatusCode("提交任务", httpResp.Header().Get("X-Api-Status-Code")); err != nil {
		return "", err
	}
	return taskID, nil
}

// Query 查询识别任务：POST query，body {"id":taskID}。status 原样返回（Queuing|Running|Completed|Failed）；
// Completed 时解析 result（text/utterances）与 audio_info.duration，utterances 缺失时 Segments 为空。
func (c *ASRAUCClient) Query(ctx context.Context, taskID string) (ASRNostreamResp, string, error) {
	headers := sauc.NewAuthHeaderFrom(c.cred.APIKey, c.cred.AppID, c.cred.AccessToken, asrAUCResourceID)

	httpResp, err := c.resty.R().
		SetContext(ctx).
		SetHeaders(aucHeaderMap(headers)).
		SetBody(map[string]string{"id": taskID}).
		Post(asrAUCQueryPath)
	if err != nil {
		return ASRNostreamResp{}, "", fmt.Errorf("查询火山 ASR 任务失败: %w", err)
	}
	// Query 响应头未完全核实：X-Api-Status-Code 非空时校验（鉴权类 45xxxx → ErrAuth），
	// 为空则跳过，交由 body 任务状态表达。
	if err := checkAUCStatusCode("查询任务", httpResp.Header().Get("X-Api-Status-Code")); err != nil {
		return ASRNostreamResp{}, "", err
	}

	var apiResp asrAUCQueryResp
	if err := json.Unmarshal(httpResp.Body(), &apiResp); err != nil {
		return ASRNostreamResp{}, "", fmt.Errorf("解析火山 ASR 查询响应失败: %w", err)
	}
	out := ASRNostreamResp{
		Text:       apiResp.Result.Text,
		DurationMS: apiResp.AudioInfo.Duration,
	}
	for _, u := range apiResp.Result.Utterances {
		out.Segments = append(out.Segments, ASRSegment{
			Text:    u.Text,
			StartMS: u.StartTime,
			EndMS:   u.EndTime,
		})
	}
	return out, apiResp.Status, nil
}

// aucHeaderMap 将 http.Header 转为 resty SetHeaders 需要的 map[string]string（每键取首值）。
func aucHeaderMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}

// checkAUCStatusCode 校验响应头 X-Api-Status-Code（沿用 tts_client 错误映射风格）：
// 45xxxx 鉴权类 → ErrAuth 包装；其余非 20000000 → 中文错误含 code；空（未返回）视为通过。
func checkAUCStatusCode(op, code string) error {
	if code == "" || code == asrAUCCodeOK {
		return nil
	}
	if strings.HasPrefix(code, "45") {
		return fmt.Errorf("%w: 火山 ASR %s被拒绝(X-Api-Status-Code %s)，请检查语音凭证配置", ErrAuth, op, code)
	}
	return fmt.Errorf("火山 ASR %s失败(X-Api-Status-Code %s)", op, code)
}
