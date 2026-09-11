package service

import "testing"

func TestNewRegistersTools(t *testing.T) {
	svc := newTestService(t)
	metas := svc.Registry().List()
	if len(metas) != 1 {
		t.Fatalf("registered tools = %d, want 1", len(metas))
	}
	if metas[0].Name != "tts" {
		t.Errorf("tool = %s", metas[0].Name)
	}
	if _, ok := svc.Registry().Get("volcengine", "tts"); !ok {
		t.Error("volcengine.tts not found")
	}
}

func TestSubmitUnknownTool(t *testing.T) {
	svc := newTestService(t)
	if _, _, err := svc.Engine().SubmitSync(t.Context(), "volcengine", "asr", map[string]any{}, nil); err == nil {
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
