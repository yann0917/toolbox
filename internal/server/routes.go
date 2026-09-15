package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider/volcengine"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/subtitle"
	"github.com/yann0917/toolbox/internal/task"
)

func (s *Server) Handler() http.Handler {
	r := gin.Default()
	gin.SetMode(gin.ReleaseMode)
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { ok(c, gin.H{"status": "ok"}) })
		api.GET("/tools", s.listTools)
		api.POST("/uploads", s.uploadFile)
		api.GET("/uploads/:id/stream", s.streamUpload)
		api.POST("/tasks", s.createTask)
		api.GET("/tasks", s.listTasks)
		api.GET("/tasks/:id", s.getTask)
		api.DELETE("/tasks/:id", s.deleteTask)
		api.POST("/tasks/:id/cancel", s.cancelTask)
		api.POST("/tasks/:id/rerun", s.rerunTask)
		api.GET("/artifacts/:id/stream", s.streamArtifact)
		api.GET("/artifacts/:id/download", s.downloadArtifact)
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.putSettings)
		api.POST("/settings/test-connection", s.testConnection)
		api.GET("/voices", s.listVoices)
		api.GET("/dicts", s.listDicts)
		api.GET("/search", s.searchTasks)
		api.POST("/subtitles/prepare", s.prepareSubtitles)
		api.POST("/subtitles/export", s.exportSubtitles)
		api.GET("/subtitles/presets", s.listSubtitlePresets)
		api.GET("/minutes/templates", s.listMinutesTemplates)
		api.GET("/minutes/:id/export", s.exportMinutes)
		if s.mcpHandler != nil {
			// MCP Streamable HTTP：GET(SSE)/POST/DELETE 全由 handler 处理，不套 JSON 包络
			api.Any("/mcp", gin.WrapH(s.mcpHandler))
		}
		api.GET("/ws", func(c *gin.Context) { s.hub.serveWS(c.Writer, c.Request, s.snapshotJSON) })
	}
	return r
}

func (s *Server) listTools(c *gin.Context) {
	out := []toolDTO{}
	for _, t := range s.svc.Registry().List() {
		tool, _ := s.svc.Registry().Get(t.Provider, t.Name)
		out = append(out, toolDTO{Meta: t, ParamSpecs: tool.ParamSpecs()})
	}
	ok(c, out)
}

type createTaskReq struct {
	Provider      string         `json:"provider"`
	Tool          string         `json:"tool"`
	Params        map[string]any `json:"params"`
	FileIDs       []string       `json:"file_ids"`       // /api/uploads 返回的上传文件 id
	ArtifactInput string         `json:"artifact_input"` // 已有产物 id（跨工具联动：如分离人声轨送 ASR）
}

func (s *Server) createTask(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Provider == "" || req.Tool == "" {
		fail(c, CodeBadRequest, "参数错误：provider/tool 必填")
		return
	}
	// _out 是 CLI 内部约定（仅 cmd/toolbox 显式设置产物输出路径），
	// Web 用户不可通过 params 透传，否则 TTS Tool 会把产物写到服务器任意路径。
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	delete(req.Params, "_out")
	// artifact_input → 已有产物输入（跨工具联动）：与 file_ids 互斥，产物文件须真实存在
	//（artifactAbsPath 已做 IsAbs/jail 防御）。key 固定 "audio" 交给 Engine 走本地文件通道。
	var files map[string]string
	if req.ArtifactInput != "" {
		if len(req.FileIDs) > 0 {
			fail(c, CodeBadRequest, "artifact_input 与 file_ids 只能提供其一")
			return
		}
		a, err := s.svc.DB().GetArtifact(req.ArtifactInput)
		if err == store.ErrNotFound {
			fail(c, CodeNotFound, "产物不存在")
			return
		} else if err != nil {
			failErr(c, err)
			return
		}
		abs, err := s.artifactAbsPath(a.Path)
		if err != nil {
			fail(c, CodeNotFound, "产物文件缺失")
			return
		}
		files = map[string]string{"audio": abs}
	} else if len(req.FileIDs) > 0 {
		// file_ids → 上传文件绝对路径（key 固定 "audio"），交给 Engine 走本地文件通道。
		f, err := fileIDsToFiles(s.svc.Config().DataDir, req.FileIDs)
		if err != nil {
			if errors.Is(err, errInvalidFileID) {
				fail(c, CodeBadRequest, err.Error())
			} else {
				fail(c, CodeNotFound, err.Error())
			}
			return
		}
		files = f
	}
	id, err := s.svc.Engine().SubmitRef(req.Provider, req.Tool, req.Params, files, &task.InputRef{
		FileIDs:       req.FileIDs,
		ArtifactInput: req.ArtifactInput,
	})
	if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"task_id": id})
}

func (s *Server) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	items, total, err := s.svc.DB().ListTasks(c.Query("provider"), nil, size, (page-1)*size)
	if err != nil {
		failErr(c, err)
		return
	}
	dtos := make([]taskDTO, 0, len(items))
	for _, t := range items {
		dtos = append(dtos, toTaskDTO(t))
	}
	ok(c, gin.H{"items": dtos, "total": total})
}

func (s *Server) getTask(c *gin.Context) {
	t, err := s.svc.DB().GetTask(c.Param("id"))
	if err == store.ErrNotFound {
		fail(c, CodeNotFound, "任务不存在")
		return
	} else if err != nil {
		failErr(c, err)
		return
	}
	arts, err := s.svc.DB().ListArtifacts(t.ID)
	if err != nil {
		failErr(c, err)
		return
	}
	adtos := make([]artifactDTO, 0, len(arts))
	for _, a := range arts {
		adtos = append(adtos, toArtifactDTO(a))
	}
	ok(c, gin.H{"task": toTaskDTO(*t), "artifacts": adtos})
}

func (s *Server) deleteTask(c *gin.Context) {
	if err := s.svc.DB().DeleteTask(c.Param("id")); err == store.ErrNotFound {
		fail(c, CodeNotFound, "任务不存在")
		return
	} else if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) cancelTask(c *gin.Context) {
	if err := s.svc.Engine().Cancel(c.Param("id")); err != nil {
		fail(c, CodeBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"ok": true})
}

// rerunTask 克隆原任务重新提交：params 原样回传（引擎重新校验），输入引用按
// Task.Input 重新解析——上传文件/产物可能已被清理，缺失时在提交前明确报错。
func (s *Server) rerunTask(c *gin.Context) {
	old, err := s.svc.DB().GetTask(c.Param("id"))
	if err == store.ErrNotFound {
		fail(c, CodeNotFound, "任务不存在")
		return
	} else if err != nil {
		failErr(c, err)
		return
	}
	if _, ok := s.svc.Registry().Get(old.Provider, old.Tool); !ok {
		fail(c, CodeBadRequest, fmt.Sprintf("工具 %s.%s 不可用（凭证未配置或已下线），无法重跑", old.Provider, old.Tool))
		return
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(old.Params), &params); err != nil {
		fail(c, CodeBadRequest, "原任务参数已损坏，无法重跑")
		return
	}
	if params == nil {
		params = map[string]any{}
	}
	delete(params, "_out")
	var ref task.InputRef
	if old.Input != "" {
		_ = json.Unmarshal([]byte(old.Input), &ref)
	}
	var files map[string]string
	if ref.ArtifactInput != "" {
		a, err := s.svc.DB().GetArtifact(ref.ArtifactInput)
		if err == store.ErrNotFound {
			fail(c, CodeNotFound, "原输入产物已被删除，无法重跑")
			return
		} else if err != nil {
			failErr(c, err)
			return
		}
		abs, err := s.artifactAbsPath(a.Path)
		if err != nil {
			fail(c, CodeNotFound, "原输入产物文件缺失，无法重跑")
			return
		}
		files = map[string]string{"audio": abs}
	} else if len(ref.FileIDs) > 0 {
		f, err := fileIDsToFiles(s.svc.Config().DataDir, ref.FileIDs)
		if err != nil {
			fail(c, CodeNotFound, "原上传文件已不存在，无法重跑："+err.Error())
			return
		}
		files = f
	}
	id, err := s.svc.Engine().SubmitRef(old.Provider, old.Tool, params, files, &ref)
	if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"task_id": id})
}

// artifactAbsPath 将产物相对路径解析到 data 目录下，防止路径穿越。
// Task 7 审查修正：CLI --out 重定向时产物路径可为绝对路径，直接使用；
// 相对路径才拼接到 data 目录。
// Task 9 审查修正：相对路径必须封闭在 data 目录内，`../` 逃逸一律拒绝。
func (s *Server) artifactAbsPath(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		// _out 契约：CLI 显式指定的绝对路径产物
		if _, err := os.Stat(rel); err != nil {
			return "", err
		}
		return rel, nil
	}
	abs := filepath.Join(s.svc.Config().DataDir, rel)
	dataRoot := filepath.Clean(s.svc.Config().DataDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(abs)+string(os.PathSeparator), dataRoot) {
		return "", fmt.Errorf("非法产物路径: %s", rel)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}

// 注意：stream/download 是二进制流端点，不套 JSON 包络，按真实 HTTP 语义返回。

func (s *Server) streamArtifact(c *gin.Context) {
	a, err := s.svc.DB().GetArtifact(c.Param("id"))
	if err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "产物不存在"})
		return
	}
	abs, err := s.artifactAbsPath(a.Path)
	if err != nil {
		c.JSON(404, gin.H{"error": "产物文件缺失"})
		return
	}
	c.Header("Accept-Ranges", "bytes")
	http.ServeFile(c.Writer, c.Request, abs)
}

func (s *Server) downloadArtifact(c *gin.Context) {
	a, err := s.svc.DB().GetArtifact(c.Param("id"))
	if err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "产物不存在"})
		return
	}
	abs, err := s.artifactAbsPath(a.Path)
	if err != nil {
		c.JSON(404, gin.H{"error": "产物文件缺失"})
		return
	}
	c.FileAttachment(abs, a.Filename)
}

func (s *Server) getSettings(c *gin.Context) {
	cfg := s.svc.Config()
	ok(c, gin.H{
		"volc": gin.H{
			"speech": gin.H{
				"app_id":           cfg.Volc.Speech.AppID,
				"has_access_token": cfg.Volc.Speech.AccessToken != "",
				"api_key":          cfg.Volc.Speech.APIKey,
			},
			"mediakit": gin.H{"has_api_key": cfg.Volc.MediaKit.APIKey != ""},
		},
		"data_dir": cfg.DataDir,
	})
}

type putSettingsReq struct {
	AppID          string `json:"app_id"`
	AccessToken    string `json:"access_token"`
	APIKey         string `json:"api_key"`
	MediaKitAPIKey string `json:"mediakit_api_key"`
}

func (s *Server) putSettings(c *gin.Context) {
	var req putSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, CodeBadRequest, "参数错误")
		return
	}
	// 持久化 + 热应用一体完成：内存配置换快照、工具实例按新凭证重注册，保存即生效。
	if err := s.svc.SaveCredentials(req.AppID, req.AccessToken, req.APIKey, req.MediaKitAPIKey); err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"ok": true, "note": "凭证已保存并即时生效"})
}

func (s *Server) testConnection(c *gin.Context) {
	msg, connOK := s.svc.TestSpeechConnection()
	// 顶层 ok/message 保持语音探测结果不变（向后兼容）；mediakit 段为 MediaKit 独立凭证探测。
	mkMsg, mkOK := s.svc.TestMediaKitConnection()
	ok(c, gin.H{"ok": connOK, "message": msg, "mediakit": gin.H{"ok": mkOK, "message": mkMsg}})
}

func (s *Server) listVoices(c *gin.Context) {
	ok(c, gin.H{"voices": volcengine.Voices()})
}

// ---- 字幕工坊（本地能力：纯 Go 解析/分句/导出，零上游 API 成本）----

// listSubtitlePresets 字幕样式预设：单一事实来源（internal/subtitle.Presets），
// 前端预览与导出参数都以此为准，避免两处颜色定义漂移。
func (s *Server) listSubtitlePresets(c *gin.Context) {
	ok(c, subtitle.Presets)
}

// searchTasks 转写全文搜索：LIKE 粗筛候选任务，解析 summary 后按分句/总结文本
// 精确命中提取片段（含时间戳，前端可跳到对应同步回放位置）。
func (s *Server) searchTasks(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		fail(c, CodeBadRequest, "参数错误：q 必填")
		return
	}
	tasks, err := s.svc.DB().SearchSucceededSummaries(q, 50)
	if err != nil {
		failErr(c, err)
		return
	}
	lower := strings.ToLower(q)
	type matchSeg struct {
		Text    string `json:"text"`
		StartMS int64  `json:"start_ms"`
		EndMS   int64  `json:"end_ms"`
	}
	type summaryJSON struct {
		Segments     []matchSeg `json:"segments"`
		SummaryText  string     `json:"summary_text"`
		MinutesTitle string     `json:"minutes_title"`
	}
	items := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		var sv summaryJSON
		if json.Unmarshal([]byte(t.Summary), &sv) != nil {
			continue
		}
		var matches []matchSeg
		for _, seg := range sv.Segments {
			if strings.Contains(strings.ToLower(seg.Text), lower) {
				matches = append(matches, seg)
				if len(matches) == 5 {
					break
				}
			}
		}
		if len(matches) == 0 {
			// LIKE 粗筛命中但分句未命中：查总结文本/标题（如妙记 summary_text），
			// 都不中则本次为键名误命中，丢弃
			hay := sv.SummaryText
			if hay == "" {
				hay = sv.MinutesTitle
			}
			if bi := strings.Index(strings.ToLower(hay), lower); bi >= 0 {
				// 按 rune 提取上下文窗口（本项目语种下 ToLower 不改变 rune 数）
				runes := []rune(hay)
				rOff := len([]rune(strings.ToLower(hay)[:bi]))
				start, end := rOff-30, rOff+len([]rune(q))+60
				if start < 0 {
					start = 0
				}
				if end > len(runes) {
					end = len(runes)
				}
				matches = append(matches, matchSeg{Text: string(runes[start:end])})
			} else {
				continue
			}
		}
		items = append(items, gin.H{
			"task_id":    t.ID,
			"tool":       t.Tool,
			"created_at": t.CreatedAt.Format("2006-01-02 15:04:05"),
			"matches":    matches,
		})
	}
	ok(c, gin.H{"items": items, "total": len(items)})
}

type prepareSubtitlesReq struct {
	Mode       string `json:"mode"` // srt：解析 SRT；text：文稿分句草稿
	Content    string `json:"content"`
	DurationMS int64  `json:"duration_ms"` // text 模式的草稿总时长，按字数比例分配
}

func (s *Server) prepareSubtitles(c *gin.Context) {
	var req prepareSubtitlesReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		fail(c, CodeBadRequest, "参数错误：content 必填")
		return
	}
	var segs []subtitle.Segment
	switch req.Mode {
	case "srt":
		var err error
		if segs, err = subtitle.ParseSRT(req.Content); err != nil {
			fail(c, CodeBadRequest, err.Error())
			return
		}
	case "text":
		segs = subtitle.DraftSegments(req.Content, req.DurationMS)
	default:
		fail(c, CodeBadRequest, "mode 仅支持 srt | text")
		return
	}
	if len(segs) == 0 {
		fail(c, CodeBadRequest, "没有解析到可用内容")
		return
	}
	ok(c, gin.H{"segments": segs})
}

type exportSubtitlesReq struct {
	Segments []subtitle.Segment `json:"segments"`
	Format   string             `json:"format"` // srt | ass
	Style    struct {
		Name     string `json:"name"`
		FontSize int    `json:"font_size"`
		MarginV  int    `json:"margin_v"`
		Karaoke  *bool  `json:"karaoke"`
	} `json:"style"`
}

func (s *Server) exportSubtitles(c *gin.Context) {
	var req exportSubtitlesReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Segments) == 0 {
		fail(c, CodeBadRequest, "参数错误：segments 不能为空")
		return
	}
	var data []byte
	ct := "text/plain; charset=utf-8"
	filename := "subtitles"
	switch strings.ToLower(req.Format) {
	case "srt":
		data = subtitle.BuildSRT(req.Segments)
		filename += ".srt"
	case "ass":
		style := subtitle.PresetByName(req.Style.Name)
		if req.Style.FontSize > 0 {
			style.FontSize = req.Style.FontSize
		}
		if req.Style.MarginV > 0 {
			style.MarginV = req.Style.MarginV
		}
		if req.Style.Karaoke != nil {
			style.Karaoke = *req.Style.Karaoke
		}
		data = subtitle.BuildASS(req.Segments, style)
		ct = "text/x-ssa; charset=utf-8"
		filename += ".ass"
	default:
		fail(c, CodeBadRequest, "format 仅支持 srt | ass")
		return
	}
	if len(data) == 0 {
		fail(c, CodeBadRequest, "没有可导出的字幕内容")
		return
	}
	// 二进制流端点惯例：不套 JSON 包络，真实文件语义
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, ct, data)
}

// listDicts 命名词典清单（config.yaml dicts 段）：供工具页「从词典填入」。
// 热词/术语非敏感，原样返回；管理走 CLI（toolbox dict add/rm）。
func (s *Server) listDicts(c *gin.Context) {
	dicts, err := config.Dicts()
	if err != nil {
		failErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(dicts))
	for _, d := range dicts {
		out = append(out, gin.H{"name": d.Name, "hotwords": d.Hotwords, "terms": d.Terms})
	}
	ok(c, out)
}
