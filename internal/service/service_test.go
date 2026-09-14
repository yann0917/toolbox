package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/task"
)

func TestNewRegistersTools(t *testing.T) {
	svc := newTestService(t)
	metas := svc.Registry().List()
	if len(metas) != 8 {
		t.Fatalf("registered tools = %d, want 8", len(metas))
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

// TestSaveCredentialsHotReload 验证凭证保存即时生效：内存配置换快照、
// 工具覆盖重注册、YAML 持久化，且空值不覆盖已有凭证。
// 注意：SaveCredentials 写 $HOME/.toolbox/config.yaml，须先隔离 HOME。
func TestSaveCredentialsHotReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := newTestService(t)

	if got := svc.Config().Volc.Speech.AppID; got != "" {
		t.Fatalf("initial app id = %q, want empty", got)
	}
	if err := svc.SaveCredentials("app-1", "tok-1", "key-1", "mk-1"); err != nil {
		t.Fatal(err)
	}
	cfg := svc.Config()
	if cfg.Volc.Speech.AppID != "app-1" || cfg.Volc.Speech.AccessToken != "tok-1" ||
		cfg.Volc.Speech.APIKey != "key-1" || cfg.Volc.MediaKit.APIKey != "mk-1" {
		t.Fatalf("in-memory config not hot-applied: %+v %+v", cfg.Volc.Speech, cfg.Volc.MediaKit)
	}
	// 覆盖重注册后工具集完整，且未触发"重复注册"报错
	if _, ok := svc.Registry().Get("volcengine", "tts"); !ok {
		t.Error("volcengine.tts missing after hot reload")
	}
	if len(svc.Registry().List()) != 8 {
		t.Errorf("List len = %d, want 8", len(svc.Registry().List()))
	}
	// 持久化：重读磁盘配置与内存一致
	persisted, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Volc.Speech.AppID != "app-1" || persisted.Volc.MediaKit.APIKey != "mk-1" {
		t.Fatalf("persisted config mismatch: %+v", persisted.Volc)
	}
	// 部分保存：空值跳过，其余字段保留
	if err := svc.SaveCredentials("app-2", "", "", ""); err != nil {
		t.Fatal(err)
	}
	cfg = svc.Config()
	if cfg.Volc.Speech.AppID != "app-2" || cfg.Volc.Speech.AccessToken != "tok-1" || cfg.Volc.MediaKit.APIKey != "mk-1" {
		t.Fatalf("partial save broke existing credentials: %+v", cfg.Volc)
	}
}

// TestReloadVolcFromDisk 配置文件监听回调路径：仅凭证段跟随磁盘配置，
// 端口/数据目录是启动期属性不跟随，工具集覆盖重注册后完整。
func TestReloadVolcFromDisk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := newTestService(t)
	if err := svc.SaveCredentials("app-1", "tok-1", "", "mk-1"); err != nil {
		t.Fatal(err)
	}
	before := svc.Config().DataDir

	svc.ReloadVolc(&config.Config{
		Server:  config.ServerConfig{Port: 9999},
		DataDir: "/somewhere/else",
		Volc: config.VolcConfig{
			Speech:   config.SpeechConfig{AppID: "app-9", AccessToken: "tok-9", APIKey: "key-9"},
			MediaKit: config.MediaKitConfig{APIKey: "mk-9"},
		},
	})

	cfg := svc.Config()
	if cfg.Volc.Speech.AppID != "app-9" || cfg.Volc.Speech.APIKey != "key-9" || cfg.Volc.MediaKit.APIKey != "mk-9" {
		t.Fatalf("volc segment not applied: %+v", cfg.Volc)
	}
	if cfg.Server.Port == 9999 || cfg.DataDir != before {
		t.Errorf("启动期属性不应跟随磁盘配置: port=%d dataDir=%s", cfg.Server.Port, cfg.DataDir)
	}
	if _, ok := svc.Registry().Get("volcengine", "tts"); !ok {
		t.Error("volcengine.tts missing after reload")
	}
	if len(svc.Registry().List()) != 8 {
		t.Errorf("List len = %d, want 8", len(svc.Registry().List()))
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
