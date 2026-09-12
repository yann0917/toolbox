package volcengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yann0917/toolbox/internal/provider"
)

// ASR 异步轮询节奏（var 便于测试注入更短间隔）：起步间隔指数退避、退避上限、总超时。
var (
	asrPollInterval = 2 * time.Second
	asrPollMax      = 30 * time.Second
	asrPollTimeout  = 10 * time.Minute
)

// ASRTool 语音识别工具：本地文件走 WS 同步通道（Files["audio"]），
// 公网 URL 走异步 submit/query 通道（Params["url"]），产物为转写文本与 SRT 字幕。
type ASRTool struct {
	ws     *ASRClient
	auc    *ASRAUCClient
	cred   SpeechCred
	outDir string // 产物写入目录（默认 ~/.toolbox/data，由构造方注入）
}

func NewASRTool(cred SpeechCred, outDir string) *ASRTool {
	return &ASRTool{
		ws:     NewASRClient(cred),
		auc:    NewASRAUCClient(cred),
		cred:   cred,
		outDir: outDir,
	}
}

func (t *ASRTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{
		Provider:    "volcengine",
		Name:        "asr",
		Title:       "语音识别",
		Description: "音频转文字，支持本地文件与公网 URL，输出分句时间戳与 SRT 字幕",
		Group:       "语音",
	}
}

func (t *ASRTool) ParamSpecs() []provider.ParamSpec {
	return []provider.ParamSpec{
		{Key: "url", Label: "音频 URL", Type: provider.ParamString,
			Placeholder: "公网音频 URL，与上传文件二选一", Group: "输入"},
		{Key: "hotwords", Label: "热词", Type: provider.ParamString,
			Placeholder: "逗号分隔热词", Group: "输入"},
		{Key: "language", Label: "语言", Type: provider.ParamString,
			Default: "zh-CN", Group: "输入"},
		{Key: "srt", Label: "生成 SRT 字幕", Type: provider.ParamBool,
			Default: true, Group: "输出"},
	}
}

func (t *ASRTool) Run(ctx context.Context, in provider.TaskInput, report provider.ProgressReporter) (provider.TaskOutput, error) {
	// 参数校验先行：缺少输入、格式不受支持属参数错误（退出码 2），
	// 不应被凭证校验（退出码 4）掩盖。
	audioPath := in.Files["audio"]
	if audioPath == "" && paramString(in.Params, "url") == "" {
		return provider.TaskOutput{}, fmt.Errorf("缺少输入：请上传音频文件或提供音频 URL")
	}
	var format string
	if audioPath != "" {
		var err error
		if format, err = audioFormatOf(audioPath); err != nil {
			return provider.TaskOutput{}, err
		}
	}
	if err := t.cred.Validate(); err != nil {
		return provider.TaskOutput{}, err
	}

	var (
		resp   ASRNostreamResp
		source string
	)
	switch audioPath := in.Files["audio"]; {
	case audioPath != "": // 本地文件模式：WS 同步识别
		source = "file"
		audio, err := os.ReadFile(audioPath)
		if err != nil {
			return provider.TaskOutput{}, fmt.Errorf("读取音频文件失败: %w", err)
		}
		report(20, "正在识别音频（本地文件）", nil)
		resp, err = t.ws.Recognize(ctx, ASRNostreamReq{
			Audio:    audio,
			Format:   format,
			Language: paramString(in.Params, "language"),
			Hotwords: paramString(in.Params, "hotwords"),
		})
		if err != nil {
			return provider.TaskOutput{}, err
		}
	case paramString(in.Params, "url") != "": // URL 模式：异步 submit + 轮询
		source = "url"
		report(10, "提交异步识别任务", nil)
		taskID, err := t.auc.Submit(ctx, paramString(in.Params, "url"))
		if err != nil {
			return provider.TaskOutput{}, err
		}
		report(30, "等待识别结果", map[string]any{"task_id": taskID})
		resp, err = t.pollAUC(ctx, taskID)
		if err != nil {
			return provider.TaskOutput{}, err
		}
	default:
		return provider.TaskOutput{}, fmt.Errorf("缺少输入：请上传音频文件或提供音频 URL")
	}
	report(90, "保存识别结果", nil)
	return t.saveArtifacts(in, resp, source)
}

// pollAUC 轮询异步识别任务：起步 asrPollInterval 指数退避至 asrPollMax，
// 总时长 asrPollTimeout 兜底（空 status 等中间态靠它终止）；ctx 取消优先返回。
// 终态：Completed → 结果；Failed → 报错。
func (t *ASRTool) pollAUC(ctx context.Context, taskID string) (ASRNostreamResp, error) {
	deadline := time.Now().Add(asrPollTimeout)
	interval := asrPollInterval
	for {
		if err := ctx.Err(); err != nil {
			return ASRNostreamResp{}, fmt.Errorf("ASR 识别已取消: %w", err)
		}
		if time.Now().After(deadline) {
			return ASRNostreamResp{}, fmt.Errorf("等待火山 ASR 识别结果超时，请稍后重试")
		}
		resp, status, err := t.auc.Query(ctx, taskID)
		if err != nil {
			return ASRNostreamResp{}, err
		}
		switch status {
		case "Completed":
			return resp, nil
		case "Failed":
			return ASRNostreamResp{}, fmt.Errorf("上游识别失败（任务 %s）", taskID)
		}
		select {
		case <-ctx.Done():
			return ASRNostreamResp{}, fmt.Errorf("ASR 识别已取消: %w", ctx.Err())
		case <-time.After(interval):
		}
		interval *= 2
		if interval > asrPollMax {
			interval = asrPollMax
		}
	}
}

// saveArtifacts 落盘转写文本（asr/<uuid>.txt）与 SRT 字幕（asr/<uuid>.srt，
// srt 参数默认开启且分句非空时生成），并按 M2 契约处理 _out 重定向。
func (t *ASRTool) saveArtifacts(in provider.TaskInput, resp ASRNostreamResp, source string) (provider.TaskOutput, error) {
	srtContent := ""
	if asrSRTEnabled(in.Params) && len(resp.Segments) > 0 {
		srtContent = BuildSRT(resp.Segments)
	}

	reqID := uuid.NewString()
	txtPath := filepath.Join("asr", reqID+".txt")
	// _out 参数（CLI --out）重定向产物路径；不进 ParamSpecs，属机器约定。
	if outParam, ok := in.Params["_out"].(string); ok && outParam != "" {
		txtPath = outParam
	}
	// srt 路径跟随 txt 路径：仅换扩展名（_out 为绝对路径时 srt 同为绝对；相对时在相对段上替换）。
	srtPath := strings.TrimSuffix(txtPath, filepath.Ext(txtPath)) + ".srt"

	txtAbs := txtPath
	if !filepath.IsAbs(txtAbs) {
		txtAbs = filepath.Join(t.outDir, txtPath)
		txtPath, _ = filepath.Rel(t.outDir, txtAbs)
	}
	srtAbs := srtPath
	if !filepath.IsAbs(srtAbs) {
		srtAbs = filepath.Join(t.outDir, srtPath)
		srtPath, _ = filepath.Rel(t.outDir, srtAbs)
	}
	if err := os.MkdirAll(filepath.Dir(txtAbs), 0o755); err != nil {
		return provider.TaskOutput{}, fmt.Errorf("创建产物目录失败: %w", err)
	}
	if err := os.WriteFile(txtAbs, []byte(resp.Text), 0o644); err != nil {
		return provider.TaskOutput{}, fmt.Errorf("写入转写文本失败: %w", err)
	}

	arts := []provider.Artifact{{
		Kind: "transcript", Path: txtPath, Format: "txt",
		Size: int64(len(resp.Text)), DurationMS: resp.DurationMS,
	}}
	if srtContent != "" {
		if err := os.WriteFile(srtAbs, []byte(srtContent), 0o644); err != nil {
			return provider.TaskOutput{}, fmt.Errorf("写入 SRT 字幕失败: %w", err)
		}
		arts = append(arts, provider.Artifact{
			Kind: "subtitle", Path: srtPath, Format: "srt", Size: int64(len(srtContent)),
		})
	}

	segs := make([]map[string]any, 0, len(resp.Segments))
	for _, s := range resp.Segments {
		segs = append(segs, map[string]any{"text": s.Text, "start_ms": s.StartMS, "end_ms": s.EndMS})
	}
	return provider.TaskOutput{
		Artifacts: arts,
		Summary: map[string]any{
			"segments":    segs,
			"duration_ms": resp.DurationMS,
			"source":      source,
		},
	}, nil
}

// audioFormatOf 由扩展名推断音频格式：mp3/wav/ogg/pcm 之外（m4a/aac/flac/mp4 及未知扩展名）
// 一律报「暂不支持」（CLI 侧映射参数错误退出码 2）。
func audioFormatOf(path string) (string, error) {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	switch strings.ToLower(ext) {
	case "mp3", "wav", "ogg", "pcm":
		return strings.ToLower(ext), nil
	}
	if ext == "" {
		return "", fmt.Errorf("暂不支持该音频格式（支持 mp3/wav/ogg/pcm）")
	}
	return "", fmt.Errorf("暂不支持该音频格式 .%s（支持 mp3/wav/ogg/pcm）", ext)
}

// asrSRTEnabled 读取 srt 参数（默认开启；兼容 bool 与字符串 "false"）。
func asrSRTEnabled(params map[string]any) bool {
	switch v := params["srt"].(type) {
	case bool:
		return v
	case string:
		return !strings.EqualFold(v, "false")
	}
	return true
}

// paramString 取字符串参数（缺失或类型不符返回空串）。
func paramString(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return s
}
