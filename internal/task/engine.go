// Package task 实现任务引擎：参数校验、并发执行、状态落库、事件通知。
package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type Event struct {
	Type      string              `json:"type"` // progress|done|error|canceled|task.snapshot
	TaskID    string              `json:"task_id"`
	Provider  string              `json:"provider,omitempty"` // 归属工具，前端全局通知/展示用
	Tool      string              `json:"tool,omitempty"`
	Progress  int                 `json:"progress"`
	Note      string              `json:"note"`
	Error     string              `json:"error"`
	Detail    map[string]any      `json:"detail,omitempty"` // 仅 progress 事件携带的工具自定义展示数据（如播客对话流轮次）
	Artifacts []provider.Artifact `json:"artifacts"`
}

type Engine struct {
	db      *store.DB
	reg     *provider.Registry
	dataDir string
	sem     chan struct{}
	notify  func(Event)
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func New(db *store.DB, reg *provider.Registry, dataDir string, concurrency int, notify func(Event)) *Engine {
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Engine{
		db: db, reg: reg, dataDir: dataDir,
		sem:     make(chan struct{}, concurrency),
		notify:  notify,
		cancels: map[string]context.CancelFunc{},
	}
}

// InputRef 记录 Web 提交边界的原始输入引用（file_ids/artifact_input 在进引擎前
// 就被解析成本地路径，不落 Params）；重跑与前端回放依赖它溯源。CLI 直传本地路径，无需引用。
type InputRef struct {
	FileIDs       []string `json:"file_ids,omitempty"`
	ArtifactInput string   `json:"artifact_input,omitempty"`
}

func (r *InputRef) empty() bool { return r == nil || (len(r.FileIDs) == 0 && r.ArtifactInput == "") }

// emit 在引擎 goroutine 内同步调用 notify 回调。
// 契约：notify 由任务执行 goroutine 同步触发，订阅方必须非阻塞
// （使用带缓冲的 channel 并配合丢弃策略），不得在回调内做耗时操作或再回调引擎。
func (e *Engine) emit(ev Event) {
	if e.notify != nil {
		e.notify(ev)
	}
}

// validate 检查必填参数；返回错误消息包含缺失参数 key。
func validate(t provider.Tool, params map[string]any) error {
	for _, spec := range t.ParamSpecs() {
		if !spec.Required {
			continue
		}
		v, ok := params[spec.Key]
		if !ok || v == nil || fmt.Sprint(v) == "" {
			return fmt.Errorf("缺少必填参数: %s", spec.Key)
		}
	}
	return nil
}

func (e *Engine) createTask(providerName, toolName string, params map[string]any, ref *InputRef) (*store.Task, provider.Tool, error) {
	tool, ok := e.reg.Get(providerName, toolName)
	if !ok {
		return nil, nil, fmt.Errorf("未知工具: %s.%s", providerName, toolName)
	}
	if err := validate(tool, params); err != nil {
		return nil, nil, err
	}
	raw, _ := json.Marshal(params)
	t := &store.Task{
		ID: uuid.NewString(), Provider: providerName, Tool: toolName,
		Status: store.StatusPending, Params: string(raw),
	}
	if !ref.empty() {
		refRaw, _ := json.Marshal(ref)
		t.Input = string(refRaw)
	}
	if err := e.db.CreateTask(t); err != nil {
		return nil, nil, err
	}
	return t, tool, nil
}

func (e *Engine) run(ctx context.Context, t *store.Task, tool provider.Tool, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error) {
	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancels[t.ID] = cancel
	e.mu.Unlock()
	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.cancels, t.ID)
		e.mu.Unlock()
	}()

	start := time.Now() // --json 契约要求 cost_ms 为任务总耗时

	t.Status = store.StatusRunning
	_ = e.db.UpdateTask(t)
	e.emit(Event{Type: "progress", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool, Progress: 0, Note: "任务开始"})

	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	report := func(progress int, note string, detail map[string]any) {
		t.Progress = progress
		t.ProgressNote = note
		_ = e.db.UpdateTask(t)
		// detail 可能为 nil（多数工具不传），omitempty 保证 JSON 输出向后兼容。
		e.emit(Event{Type: "progress", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool, Progress: progress, Note: note, Detail: detail})
	}

	out, runErr := tool.Run(ctx, provider.TaskInput{Params: params, Files: files}, report)

	var saved []store.Artifact
	for _, a := range out.Artifacts {
		raw, _ := json.Marshal(a.Meta)
		sa := store.Artifact{
			ID: uuid.NewString(), TaskID: t.ID, Kind: a.Kind, Path: a.Path,
			Filename: filepath.Base(a.Path), Format: a.Format,
			Size: a.Size, DurationMS: a.DurationMS, Meta: string(raw),
		}
		if err := e.db.CreateArtifact(&sa); err != nil {
			// 产物落库失败也必须进入终态，否则任务会永久停留在 running 且不发终态事件。
			t.Status = store.StatusFailed
			t.Error = fmt.Sprintf("保存产物失败: %v", err)
			t.CostMS = time.Since(start).Milliseconds()
			_ = e.db.UpdateTask(t)
			e.emit(Event{Type: "error", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool, Error: t.Error})
			return t, saved, fmt.Errorf("保存产物失败: %w", err)
		}
		saved = append(saved, sa)
	}

	var ev Event
	switch {
	case runErr == nil:
		t.Status = store.StatusSucceeded
		t.Progress = 100
		if out.Summary != nil {
			raw, _ := json.Marshal(out.Summary)
			t.Summary = string(raw)
		}
		ev = Event{Type: "done", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool, Progress: 100, Artifacts: out.Artifacts}
	case errors.Is(runErr, context.Canceled):
		t.Status = store.StatusCanceled
		ev = Event{Type: "canceled", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool}
	default:
		t.Status = store.StatusFailed
		t.Error = runErr.Error()
		ev = Event{Type: "error", TaskID: t.ID, Provider: t.Provider, Tool: t.Tool, Error: runErr.Error()}
	}
	// 先落库终态，再发终态事件：订阅方收到事件时 DB 状态已就绪。
	// 任务开始处的「先 UpdateTask 再 emit」与本处顺序保持一致。
	t.CostMS = time.Since(start).Milliseconds()
	_ = e.db.UpdateTask(t)
	e.emit(ev)
	return t, saved, runErr
}

func (e *Engine) Submit(providerName, toolName string, params map[string]any, files map[string]string) (string, error) {
	return e.SubmitRef(providerName, toolName, params, files, nil)
}

// SubmitRef 与 Submit 等价，额外把原始输入引用落库（Web 提交边界使用）。
func (e *Engine) SubmitRef(providerName, toolName string, params map[string]any, files map[string]string, ref *InputRef) (string, error) {
	t, tool, err := e.createTask(providerName, toolName, params, ref)
	if err != nil {
		return "", err
	}
	go func() { _, _, _ = e.run(context.Background(), t, tool, params, files) }()
	return t.ID, nil
}

func (e *Engine) SubmitSync(ctx context.Context, providerName, toolName string, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error) {
	t, tool, err := e.createTask(providerName, toolName, params, nil)
	if err != nil {
		return nil, nil, err
	}
	return e.run(ctx, t, tool, params, files)
}

func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	cancel, ok := e.cancels[id]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("任务不在运行中: %s", id)
	}
	cancel()
	return nil
}
