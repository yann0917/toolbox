package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/task"
)

func TestNewRegistersTools(t *testing.T) {
	svc := newTestService(t)
	metas := svc.Registry().List()
	if len(metas) != 4 {
		t.Fatalf("registered tools = %d, want 4", len(metas))
	}
	if _, ok := svc.Registry().Get("volcengine", "tts"); !ok {
		t.Error("volcengine.tts not found")
	}
	if _, ok := svc.Registry().Get("volcengine", "asr"); !ok {
		t.Error("volcengine.asr not found")
	}
	if _, ok := svc.Registry().Get("volcengine", "podcast"); !ok {
		t.Error("volcengine.podcast not found")
	}
	if _, ok := svc.Registry().Get("volcengine", "separate"); !ok {
		t.Error("volcengine.separate not found")
	}
}

func TestSubmitUnknownTool(t *testing.T) {
	svc := newTestService(t)
	if _, _, err := svc.Engine().SubmitSync(t.Context(), "volcengine", "nonexistent", map[string]any{}, nil); err == nil {
		t.Error("unknown tool should fail")
	}
}

// newTestService 以临时目录构造 Service 并启动引擎（Engine() 未启动会 panic）。
func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	svc.StartEngine(nil, 1)
	return svc
}

// fakeTool 记录引擎透传的 Files，用于验证 TaskInput.Files 链路。
type fakeTool struct{ gotFiles map[string]string }

func (f *fakeTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{Provider: "fake", Name: "fake", Title: "Fake"}
}
func (f *fakeTool) ParamSpecs() []provider.ParamSpec { return nil }
func (f *fakeTool) Run(_ context.Context, in provider.TaskInput, _ provider.ProgressReporter) (provider.TaskOutput, error) {
	f.gotFiles = in.Files
	return provider.TaskOutput{}, nil
}

// TestSubmitSyncWithFiles 验证引擎把 files 原样透传给工具的 TaskInput.Files。
func TestSubmitSyncWithFiles(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	tool := &fakeTool{}
	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}
	e := task.New(db, reg, t.TempDir(), 1, nil)

	const audioPath = "/tmp/whatever.mp3"
	res, _, err := e.SubmitSync(t.Context(), "fake", "fake", map[string]any{}, map[string]string{"audio": audioPath})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != store.StatusSucceeded {
		t.Fatalf("status = %s, want succeeded (err=%s)", res.Status, res.Error)
	}
	if got := tool.gotFiles["audio"]; got != audioPath {
		t.Errorf("Files[audio] = %q, want %q", got, audioPath)
	}
}
