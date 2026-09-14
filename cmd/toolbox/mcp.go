package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider/volcengine"
	"github.com/yann0917/toolbox/internal/service"
)

// MCP server：把八个语音能力以 stdio MCP 工具暴露给 AI Agent（Claude Code 等）。
// 复用 CLI 的执行核心 runToolCore，输出与 `--json` 同一契约（docs/json-contract.md）。
// 客户端配置见 docs/mcp.md。

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "启动 MCP server（stdio），供 AI Agent 调用全部能力",
		Long: `以 Model Context Protocol stdio 模式运行 toolbox，暴露 toolbox_* 工具族：
语音合成（同步/长文本/流式）、语音识别、播客、人声分离、机器翻译、语音妙记与音色查询。
凭证复用 ~/.toolbox/config.yaml；工具输出与 CLI --json 同一契约。
配置示例（Claude Code）：
  {"mcpServers": {"toolbox": {"command": "/path/to/toolbox", "args": ["mcp"]}}}`,
		RunE: func(c *cobra.Command, args []string) error {
			return runMCP(c.Context())
		},
	}
}

// mcpRunner 串行执行工具调用：sqlite 写入按单路处理，避免 MCP 客户端并发
// 调用触发 database is locked。代价是分钟级任务（妙记/播客）会阻塞后续调用。
type mcpRunner struct {
	svc *service.Service
	mu  sync.Mutex
}

func (r *mcpRunner) run(ctx context.Context, providerName, toolName string, params map[string]any, files map[string]string) (jsonResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return runToolCore(ctx, r.svc, providerName, toolName, params, files)
}

func runMCP(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := service.New(cfg)
	if err != nil {
		return err
	}
	defer svc.Close()
	svc.StartEngine(nil, 2) // stdio 下进度事件无消费者，不订阅
	runner := &mcpRunner{svc: svc}

	server := mcp.NewServer(&mcp.Implementation{Name: "toolbox", Version: version}, nil)
	addMCPTools(server, runner)
	eprintf("toolbox MCP server 已就绪（stdio）\n")
	return server.Run(ctx, &mcp.StdioTransport{})
}

func addMCPTools(server *mcp.Server, r *mcpRunner) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_tts",
		Description: "语音合成：短文本转语音（同步秒级）。计费按字符数。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpTTSIn) (*mcp.CallToolResult, jsonResult, error) {
		if strings.TrimSpace(in.Text) == "" {
			return nil, jsonResult{}, fmt.Errorf("text 必填")
		}
		params := map[string]any{"text": in.Text}
		if in.Voice != "" {
			params["voice"] = in.Voice
		}
		if in.Format != "" {
			params["format"] = in.Format
		}
		if in.SpeedRatio != 0 {
			params["speed_ratio"] = in.SpeedRatio
		}
		if in.VolumeRatio != 0 {
			params["volume_ratio"] = in.VolumeRatio
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "tts", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_tts_long",
		Description: "长文本语音合成：≤10 万字，有声书级异步合成，耗时时长正相关。timestamps=true 额外产出 SRT 字幕。计费按字符数。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpTTSLongIn) (*mcp.CallToolResult, jsonResult, error) {
		if strings.TrimSpace(in.Text) == "" {
			return nil, jsonResult{}, fmt.Errorf("text 必填")
		}
		params := map[string]any{"text": in.Text}
		if in.Voice != "" {
			params["voice"] = in.Voice
		}
		if in.Format != "" {
			params["format"] = in.Format
		}
		applySharedTTS(&in.SharedTTSOpts, params)
		if in.Timestamps {
			params["timestamps"] = true
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "tts_long", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_tts_stream",
		Description: "流式语音合成：低延迟，20 语种 8 方言；context_text 传语音指令（如「用粤语说」「特别愤怒」），subtitle=true 产出字级字幕 SRT。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpTTSStreamIn) (*mcp.CallToolResult, jsonResult, error) {
		if strings.TrimSpace(in.Text) == "" {
			return nil, jsonResult{}, fmt.Errorf("text 必填")
		}
		params := map[string]any{"text": in.Text}
		if in.Voice != "" {
			params["voice"] = in.Voice
		}
		if in.Format != "" {
			params["format"] = in.Format
		}
		applySharedTTS(&in.SharedTTSOpts, params)
		if in.SilenceDuration != 0 {
			params["silence_duration"] = in.SilenceDuration
		}
		if in.Subtitle {
			params["subtitle"] = true
		}
		if in.AigcWatermark {
			params["aigc_watermark"] = true
		}
		if in.ToneFidelity {
			params["tone_fidelity"] = true
		}
		if in.ExplicitDialect != "" {
			params["explicit_dialect"] = in.ExplicitDialect
		}
		if in.ContextText != "" {
			params["context_text"] = in.ContextText
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "tts_stream", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_asr",
		Description: "语音识别：本地音频 path 走一句话识别（同步秒级）；公网 URL 走录音文件识别，version=standard 异步 / idle 闲时低价 24h 内 / flash 极速秒级。输出分句时间戳与 SRT 字幕。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpASRIn) (*mcp.CallToolResult, jsonResult, error) {
		if (in.Path == "") == (in.URL == "") {
			return nil, jsonResult{}, fmt.Errorf("path 与 url 恰好提供其一")
		}
		version := in.Version
		if version == "" {
			if in.Path != "" {
				version = "sentence"
			} else {
				version = "standard"
			}
		}
		if in.Path != "" && version != "sentence" {
			return nil, jsonResult{}, fmt.Errorf("标准版/闲时版/极速版仅支持 URL 输入；本地文件请用 version=sentence（一句话识别）")
		}
		if in.URL != "" && version == "sentence" {
			return nil, jsonResult{}, fmt.Errorf("一句话识别仅支持本地音频文件；URL 请用 version=standard / idle / flash")
		}
		srt := true
		if in.Srt != nil {
			srt = *in.Srt
		}
		params := map[string]any{"srt": srt, "version": version, "language": in.Language}
		if in.Hotwords != "" {
			params["hotwords"] = in.Hotwords
		}
		var files map[string]string
		if in.Path != "" {
			if _, err := os.Stat(in.Path); err != nil {
				return nil, jsonResult{}, fmt.Errorf("音频文件不存在: %s", in.Path)
			}
			files = map[string]string{"audio": in.Path}
		} else {
			params["url"] = in.URL
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "asr", params, files)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_podcast",
		Description: "播客生成：主题文本（text）/长文本/网页 URL（url）/对话稿文件（script）生成双人对话播客音频。speakers 必填恰好 2 个音色 ID（先用 toolbox_voices 查询）。分钟级耗时，计费按字符。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpPodcastIn) (*mcp.CallToolResult, jsonResult, error) {
		params := map[string]any{}
		switch {
		case in.Script != "":
			raw, err := os.ReadFile(in.Script)
			if err != nil {
				return nil, jsonResult{}, fmt.Errorf("读取对话稿失败: %w", err)
			}
			params["script"] = string(raw)
		case in.URL != "":
			params["url"] = in.URL
		case in.Text != "":
			params["input_text"] = in.Text
		default:
			return nil, jsonResult{}, fmt.Errorf("text / url / script 必须提供其一")
		}
		if strings.Count(in.Speakers, ",") != 1 {
			return nil, jsonResult{}, fmt.Errorf("speakers 需要恰好 2 个音色 ID，逗号分隔（顺序为说话人 A、B）")
		}
		params["speakers"] = in.Speakers
		if in.Format != "" {
			params["format"] = in.Format
		}
		if in.HeadMusic {
			params["head_music"] = true
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "podcast", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_separate",
		Description: "人声背景音分离：公网音视频 URL 分离多轨。scene=audio|music 双轨（人声+背景/伴奏），drama|narrate 三轨（人声+音乐+音效）。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpSeparateIn) (*mcp.CallToolResult, jsonResult, error) {
		if in.URL == "" {
			return nil, jsonResult{}, fmt.Errorf("url 必填（MediaKit 不支持本地文件，本地文件请先上传到公网可访问存储）")
		}
		params := map[string]any{"url": in.URL, "scene": in.Scene}
		if in.Format != "" {
			params["output_format"] = in.Format
		}
		if in.OutDir != "" {
			params["_out"] = in.OutDir
		}
		res, err := r.run(ctx, "volcengine", "separate", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_translate",
		Description: "机器翻译：32 语种互译，from 缺省自动检测。terms 直传术语「原词＝译词」逗号/换行分隔可固定译法。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpTranslateIn) (*mcp.CallToolResult, jsonResult, error) {
		if strings.TrimSpace(in.Text) == "" {
			return nil, jsonResult{}, fmt.Errorf("text 必填")
		}
		if in.To == "" {
			return nil, jsonResult{}, fmt.Errorf("to 必填（目标语言代码，如 zh/en/ja/zh-Hant）")
		}
		params := map[string]any{"text": in.Text, "target_language": in.To}
		if in.From != "" {
			params["source_language"] = in.From
		}
		if in.Terms != "" {
			params["terms"] = in.Terms
		}
		if in.TableID != "" {
			params["glossary_table_id"] = in.TableID
		}
		if in.TableName != "" {
			params["glossary_table_name"] = in.TableName
		}
		if in.Out != "" {
			params["_out"] = in.Out
		}
		res, err := r.run(ctx, "volcengine", "translate", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_minutes",
		Description: "语音妙记：公网音视频 URL（≤2 小时、<1G）转结构化纪要：转写+说话人、总结、待办、章节、翻译。features 逗号分隔至少一项：summary|todo|qa|chapter|translation。同步等待分钟级耗时，计费按小时（转写+结构费）。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpMinutesIn) (*mcp.CallToolResult, jsonResult, error) {
		if in.URL == "" {
			return nil, jsonResult{}, fmt.Errorf("url 必填（公网音视频地址）")
		}
		if strings.TrimSpace(in.Features) == "" {
			return nil, jsonResult{}, fmt.Errorf("features 必填（至少一项，逗号分隔：summary|todo|qa|chapter|translation）")
		}
		params := map[string]any{"url": in.URL, "features": in.Features}
		if in.SourceLang != "" {
			params["source_lang"] = in.SourceLang
		}
		if in.TargetLang != "" {
			params["target_lang"] = in.TargetLang
		}
		if in.Speakers != 0 {
			params["speakers"] = in.Speakers
		}
		if in.Hotwords != "" {
			params["hotwords"] = in.Hotwords
		}
		if in.AllActivate != nil {
			params["all_activate"] = *in.AllActivate
		}
		if in.WordTimestamps {
			params["word_timestamps"] = true
		}
		if in.OutDir != "" {
			params["_out"] = in.OutDir
		}
		res, err := r.run(ctx, "volcengine", "minutes", params, nil)
		return nil, res, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "toolbox_voices",
		Description: "音色查询：列出火山引擎内置音色（ID/名称/性别/场景/语种），用于 tts/podcast 的 voice/speakers 参数。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpVoicesIn) (*mcp.CallToolResult, []volcengine.Voice, error) {
		filtered := make([]volcengine.Voice, 0)
		for _, v := range volcengine.Voices() {
			if in.Scene != "" && !voicesMatches(v.Scenes, in.Scene) {
				continue
			}
			if in.Lang != "" && !voicesMatches(v.Languages, in.Lang) {
				continue
			}
			if in.Gender != "" && v.Gender != in.Gender {
				continue
			}
			if in.Query != "" && !strings.Contains(v.ID, in.Query) && !strings.Contains(v.Name, in.Query) {
				continue
			}
			filtered = append(filtered, v)
		}
		return nil, filtered, nil
	})
}

// SharedTTSOpts tts_long/tts_stream 共享的合成参数（键名与 CLI 完全一致）。
type SharedTTSOpts struct {
	SampleRate       int    `json:"sample_rate,omitempty" jsonschema:"采样率 Hz: 8000/16000/22050/24000/32000/44100/48000，默认 24000"`
	SpeechRate       int    `json:"speech_rate,omitempty" jsonschema:"语速 -50-100（100 即 2 倍速）"`
	LoudnessRate     int    `json:"loudness_rate,omitempty" jsonschema:"音量 -50-100（100 即 2 倍音量）"`
	Pitch            int    `json:"pitch,omitempty" jsonschema:"音调 -12-12"`
	BitRate          int    `json:"bit_rate,omitempty" jsonschema:"比特率 bps: 64000/160000"`
	AigcWatermark    bool   `json:"aigc_watermark,omitempty" jsonschema:"音频结尾添加 AIGC 节奏标识"`
	Resource         string `json:"resource,omitempty" jsonschema:"seed-tts-2.0（默认，普通音色）| seed-icl-2.0（复刻音色）"`
	Model            string `json:"model,omitempty" jsonschema:"复刻模型版本（仅复刻音色需指定）"`
	ExplicitLanguage string `json:"explicit_language,omitempty" jsonschema:"朗读语种: zh-cn|en|es-mx|id|pt-br"`
}

func applySharedTTS(o *SharedTTSOpts, params map[string]any) {
	if o.SampleRate != 0 {
		params["sample_rate"] = o.SampleRate
	}
	if o.SpeechRate != 0 {
		params["speech_rate"] = o.SpeechRate
	}
	if o.LoudnessRate != 0 {
		params["loudness_rate"] = o.LoudnessRate
	}
	if o.Pitch != 0 {
		params["pitch"] = o.Pitch
	}
	if o.BitRate != 0 {
		params["bit_rate"] = o.BitRate
	}
	if o.AigcWatermark {
		params["aigc_watermark"] = true
	}
	if o.Resource != "" {
		params["resource"] = o.Resource
	}
	if o.Model != "" {
		params["model"] = o.Model
	}
	if o.ExplicitLanguage != "" {
		params["explicit_language"] = o.ExplicitLanguage
	}
}

type mcpTTSIn struct {
	Text        string  `json:"text" jsonschema:"要合成的文本"`
	Voice       string  `json:"voice,omitempty" jsonschema:"音色 ID，默认 zh_female_cancan_mars_bigtts（可用 toolbox_voices 查询）"`
	Format      string  `json:"format,omitempty" jsonschema:"音频格式: mp3|wav|pcm|ogg_opus，默认 mp3"`
	SpeedRatio  float64 `json:"speed_ratio,omitempty" jsonschema:"语速 0.2-3.0，默认 1.0"`
	VolumeRatio float64 `json:"volume_ratio,omitempty" jsonschema:"音量 0.2-3.0，默认 1.0"`
	Out         string  `json:"out,omitempty" jsonschema:"产物输出绝对路径（缺省写入数据目录）"`
}

type mcpTTSLongIn struct {
	Text          string `json:"text" jsonschema:"要合成的长文本（≤10 万字）"`
	Voice         string `json:"voice,omitempty" jsonschema:"音色 ID，默认 zh_female_vv_uranus_bigtts（2.0/复刻音色）"`
	Format        string `json:"format,omitempty" jsonschema:"音频格式: mp3|pcm|ogg_opus，默认 mp3"`
	Timestamps    bool   `json:"timestamps,omitempty" jsonschema:"开启时间戳，额外产出 SRT 字幕"`
	SharedTTSOpts
	Out string `json:"out,omitempty" jsonschema:"产物输出绝对路径（缺省写入数据目录）"`
}

type mcpTTSStreamIn struct {
	Text            string `json:"text" jsonschema:"要合成的文本"`
	Voice           string `json:"voice,omitempty" jsonschema:"音色 ID，默认 zh_female_cancan_mars_bigtts"`
	Format          string `json:"format,omitempty" jsonschema:"音频格式: mp3|wav|pcm|ogg_opus，默认 mp3"`
	SilenceDuration int    `json:"silence_duration,omitempty" jsonschema:"句尾静音时长 ms（0-3000）"`
	Subtitle        bool   `json:"subtitle,omitempty" jsonschema:"产出字级字幕 SRT"`
	ToneFidelity    bool   `json:"tone_fidelity,omitempty" jsonschema:"复刻音色音色保真"`
	ExplicitDialect string `json:"explicit_dialect,omitempty" jsonschema:"显式方言: yue-CN（粤语）等 8 种"`
	ContextText     string `json:"context_text,omitempty" jsonschema:"语音指令：方言/语种/情感/语速等自然语言描述"`
	SharedTTSOpts
	Out string `json:"out,omitempty" jsonschema:"产物输出绝对路径（缺省写入数据目录）"`
}

type mcpASRIn struct {
	Path     string `json:"path,omitempty" jsonschema:"本地音频文件路径（mp3/wav/ogg/pcm，走一句话识别同步秒级），与 url 二选一"`
	URL      string `json:"url,omitempty" jsonschema:"公网音频 URL，与 path 二选一"`
	Version  string `json:"version,omitempty" jsonschema:"识别版本，缺省按输入推断（path→sentence，url→standard）: sentence 一句话识别（本地文件）/ standard 标准版（URL，异步）/ idle 闲时（URL，24h 内）/ flash 极速（URL，秒级）"`
	Language string `json:"language,omitempty" jsonschema:"识别语言，留空自动识别；可选 zh-CN/en-US/ja-JP/yue-CN 等 25 种"`
	Hotwords string `json:"hotwords,omitempty" jsonschema:"热词，逗号分隔，提升专有名词识别率"`
	Srt      *bool  `json:"srt,omitempty" jsonschema:"是否额外产出 SRT 字幕，默认 true"`
	Out      string `json:"out,omitempty" jsonschema:"转写文本输出绝对路径（缺省写入数据目录）"`
}

type mcpPodcastIn struct {
	Text      string `json:"text,omitempty" jsonschema:"播客主题或长文本（与 url/script 三选一）"`
	URL       string `json:"url,omitempty" jsonschema:"网页链接，服务端联网总结后生成（三选一）"`
	Script    string `json:"script,omitempty" jsonschema:"对话稿 JSON 文件路径（三选一）"`
	Speakers  string `json:"speakers" jsonschema:"两个音色 ID，逗号分隔，顺序为说话人 A、B"`
	Format    string `json:"format,omitempty" jsonschema:"音频格式: mp3|ogg_opus|pcm|aac，默认 mp3"`
	HeadMusic bool   `json:"head_music,omitempty" jsonschema:"是否加开头音乐"`
	Out       string `json:"out,omitempty" jsonschema:"播客音频输出绝对路径（缺省写入数据目录，对话稿同路径 .json）"`
}

type mcpSeparateIn struct {
	URL    string `json:"url" jsonschema:"公网音视频 URL（不支持本地文件）"`
	Scene  string `json:"scene,omitempty" jsonschema:"分离场景: audio（默认）|music|drama|narrate"`
	Format string `json:"format,omitempty" jsonschema:"输出格式: aac|mp3（默认）|wav|m4a|flac"`
	OutDir string `json:"out_dir,omitempty" jsonschema:"音轨输出目录（缺省写入数据目录）"`
}

type mcpTranslateIn struct {
	Text      string `json:"text" jsonschema:"待翻译文本"`
	To        string `json:"to" jsonschema:"目标语言代码（32 语种，如 zh/en/ja/zh-Hant）"`
	From      string `json:"from,omitempty" jsonschema:"源语言代码，缺省自动检测"`
	Terms     string `json:"terms,omitempty" jsonschema:"直传术语：原词＝译词，逗号或换行分隔"`
	TableID   string `json:"table_id,omitempty" jsonschema:"术语表 ID（术语管理平台）"`
	TableName string `json:"table_name,omitempty" jsonschema:"术语表名称（与 table_id 二选一或同传）"`
	Out       string `json:"out,omitempty" jsonschema:"译文输出绝对路径（缺省写入数据目录）"`
}

type mcpMinutesIn struct {
	URL            string `json:"url" jsonschema:"公网音视频 URL（≤2 小时、<1G）"`
	Features       string `json:"features" jsonschema:"附加功能，逗号分隔至少一项: summary|todo|qa|chapter|translation"`
	SourceLang     string `json:"source_lang,omitempty" jsonschema:"源语种: zh_cn（默认）|en_us"`
	TargetLang     string `json:"target_lang,omitempty" jsonschema:"翻译目标语: en_us（默认）|zh_cn（features 含 translation 时生效）"`
	Speakers       int    `json:"speakers,omitempty" jsonschema:"说话人数，0 即自动识别（默认）"`
	Hotwords       string `json:"hotwords,omitempty" jsonschema:"热词，逗号分隔"`
	AllActivate    *bool  `json:"all_activate,omitempty" jsonschema:"打包计费，默认 true（false 按所选功能汇总计费）"`
	WordTimestamps bool   `json:"word_timestamps,omitempty" jsonschema:"需要字级时间序列"`
	OutDir         string `json:"out_dir,omitempty" jsonschema:"结果输出目录（缺省写入数据目录）"`
}

type mcpVoicesIn struct {
	Scene  string `json:"scene,omitempty" jsonschema:"场景筛选，如 通用|带货|扮演|客服|童声"`
	Lang   string `json:"lang,omitempty" jsonschema:"语种筛选，如 中文|英语|日语"`
	Gender string `json:"gender,omitempty" jsonschema:"性别筛选: 男|女"`
	Query  string `json:"query,omitempty" jsonschema:"音色名称/ID 关键词包含匹配"`
}
