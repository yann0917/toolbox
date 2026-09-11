// Package task 实现任务引擎：参数校验、并发执行、状态落库、事件通知。
package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type Event struct {
	Type      string // progress|done|error|canceled
	TaskID    string
	Progress  int
	Note      string
	Error     string
	Artifacts []provider.Artifact
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

func (e *Engine) createTask(providerName, toolName string, params map[string]any) (*store.Task, provider.Tool, error) {
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

	t.Status = store.StatusRunning
	_ = e.db.UpdateTask(t)
	e.emit(Event{Type: "progress", TaskID: t.ID, Progress: 0, Note: "任务开始"})

	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	report := func(progress int, note string, _ map[string]any) {
		t.Progress = progress
		t.ProgressNote = note
		_ = e.db.UpdateTask(t)
		e.emit(Event{Type: "progress", TaskID: t.ID, Progress: progress, Note: note})
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
			return t, saved, fmt.Errorf("保存产物失败: %w", err)
		}
		saved = append(saved, sa)
	}

	switch {
	case runErr == nil:
		t.Status = store.StatusSucceeded
		t.Progress = 100
		if out.Summary != nil {
			raw, _ := json.Marshal(out.Summary)
			t.Summary = string(raw)
		}
		e.emit(Event{Type: "done", TaskID: t.ID, Progress: 100, Artifacts: out.Artifacts})
	case errors.Is(runErr, context.Canceled):
		t.Status = store.StatusCanceled
		e.emit(Event{Type: "canceled", TaskID: t.ID})
	default:
		t.Status = store.StatusFailed
		t.Error = runErr.Error()
		e.emit(Event{Type: "error", TaskID: t.ID, Error: runErr.Error()})
	}
	_ = e.db.UpdateTask(t)
	return t, saved, runErr
}

func (e *Engine) Submit(providerName, toolName string, params map[string]any, files map[string]string) (string, error) {
	t, tool, err := e.createTask(providerName, toolName, params)
	if err != nil {
		return "", err
	}
	go func() { _, _, _ = e.run(context.Background(), t, tool, params, files) }()
	return t.ID, nil
}

func (e *Engine) SubmitSync(ctx context.Context, providerName, toolName string, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error) {
	t, tool, err := e.createTask(providerName, toolName, params)
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
