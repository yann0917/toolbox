package task

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type echoTool struct{ fail bool }

func (echoTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{Provider: "fake", Name: "echo", Title: "Echo"}
}
func (echoTool) ParamSpecs() []provider.ParamSpec {
	return []provider.ParamSpec{{Key: "text", Label: "文本", Type: provider.ParamText, Required: true}}
}
func (t echoTool) Run(ctx context.Context, in provider.TaskInput, report provider.ProgressReporter) (provider.TaskOutput, error) {
	report(10, "开始", nil)
	if t.fail {
		return provider.TaskOutput{}, errorFail{}
	}
	select {
	case <-ctx.Done():
		return provider.TaskOutput{}, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	report(100, "完成", nil)
	return provider.TaskOutput{
		Artifacts: []provider.Artifact{{Kind: "audio", Path: "echo/a.mp3", Format: "mp3", Size: 3}},
		Summary:   map[string]any{"text": in.Params["text"]},
	}, nil
}

type errorFail struct{}

func (errorFail) Error() string { return "boom" }

// detailTool 上报带 detail 的进度，用于验证引擎透传（如播客对话流轮次）。
type detailTool struct{}

func (detailTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{Provider: "fake", Name: "detail", Title: "Detail"}
}
func (detailTool) ParamSpecs() []provider.ParamSpec { return nil }
func (detailTool) Run(ctx context.Context, in provider.TaskInput, report provider.ProgressReporter) (provider.TaskOutput, error) {
	report(30, "第 1 轮", map[string]any{"round_id": 1, "text": "x"})
	return provider.TaskOutput{}, nil
}

func TestEventDetailPassthrough(t *testing.T) {
	var events []Event
	e := newTestEngine(t, detailTool{}, &events)
	if _, _, err := e.SubmitSync(context.Background(), "fake", "detail", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "progress" && ev.Detail["round_id"] == 1 && ev.Detail["text"] == "x" {
			found = true
		}
	}
	if !found {
		t.Fatalf("progress 事件未透传 detail: %+v", events)
	}
	// 向后兼容：detail 为 nil 的事件（含终态）不应携带该字段。
	if last := events[len(events)-1]; last.Detail != nil {
		t.Errorf("终态事件不应携带 detail: %+v", last)
	}
}

func newTestEngine(t *testing.T, tool provider.Tool, events *[]Event) *Engine {
	t.Helper()
	db, err := OpenStore(t)
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}
	return New(db, reg, t.TempDir(), 2, func(e Event) { *events = append(*events, e) })
}

func TestSubmitSyncSuccess(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{}, &events)
	task, arts, err := e.SubmitSync(context.Background(), "fake", "echo",
		map[string]any{"text": "hi"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != store.StatusSucceeded || len(arts) != 1 {
		t.Fatalf("status=%s arts=%d", task.Status, len(arts))
	}
	if arts[0].Kind != "audio" {
		t.Errorf("artifact kind = %s", arts[0].Kind)
	}
	// 计时覆盖整个 Run（echo 工具内含 10ms 延迟），取保守下限 5ms；
	// cost_ms 的真实端到端验证（--json 输出）在 Task 14 e2e 覆盖。
	if task.CostMS < 5 {
		t.Errorf("cost_ms = %d, want >= 5 (tool sleeps 10ms)", task.CostMS)
	}
	if len(events) == 0 || events[len(events)-1].Type != "done" {
		t.Errorf("last event = %+v", events)
	}
}

func TestSubmitSyncValidation(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{}, &events)
	_, _, err := e.SubmitSync(context.Background(), "fake", "echo", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "text") {
		t.Errorf("err = %v, want missing required param text", err)
	}
	_, _, err = e.SubmitSync(context.Background(), "fake", "nope", map[string]any{}, nil)
	if err == nil {
		t.Error("unknown tool should fail")
	}
}

func TestSubmitSyncFailure(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{fail: true}, &events)
	task, _, err := e.SubmitSync(context.Background(), "fake", "echo", map[string]any{"text": "x"}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if task.Status != store.StatusFailed || task.Error == "" {
		t.Fatalf("status=%s err=%q", task.Status, task.Error)
	}
}
