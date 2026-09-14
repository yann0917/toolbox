// Package service 组装配置、存储、注册表与任务引擎，是 CLI 与 Web 的唯一共享入口。
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/provider/volcengine"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/task"
)

type Service struct {
	// cfg 原子指针：凭证热加载整体换新快照，读方（设置页/连通性测试/任务提交）
	// 始终拿到一致配置，与 HTTP handler 并发无数据竞争。
	cfg    atomic.Pointer[config.Config]
	db     *store.DB
	reg    *provider.Registry
	engine *task.Engine
}

func New(cfg *config.Config) (*Service, error) {
	return newWithRoot(cfg)
}

// NewWithHome 以 home 为 ~/.toolbox 根的测试构造。
func NewWithHome(home string) (*Service, error) {
	cfg, err := loadFor(home)
	if err != nil {
		return nil, err
	}
	return newWithRoot(cfg)
}

func loadFor(home string) (*config.Config, error) {
	// 测试场景：直接构造默认配置，DataDir 指向 home/data
	return &config.Config{
		Server:  config.ServerConfig{Port: 0},
		DataDir: filepath.Join(home, "data"),
	}, nil
}

func newWithRoot(cfg *config.Config) (*Service, error) {
	dataDir := cfg.DataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	db, err := store.Open(filepath.Join(dataDir, "toolbox.db"))
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	reg := provider.NewRegistry()
	if err := volcengine.RegisterAll(reg, *cfg, dataDir); err != nil {
		return nil, err
	}
	s := &Service{db: db, reg: reg}
	s.cfg.Store(cfg)
	return s, nil
}

func (s *Service) StartEngine(notify func(task.Event), concurrency int) {
	s.engine = task.New(s.db, s.reg, s.cfg.Load().DataDir, concurrency, notify)
}

func (s *Service) Engine() *task.Engine {
	if s.engine == nil {
		panic("engine not started: call StartEngine first")
	}
	return s.engine
}

func (s *Service) DB() *store.DB                { return s.db }
func (s *Service) Registry() *provider.Registry { return s.reg }
func (s *Service) Config() *config.Config       { return s.cfg.Load() }

// SaveCredentials 将非空凭证持久化到 ~/.toolbox/config.yaml 并热应用：
// 整体换新配置快照、以新凭证覆盖重注册火山工具，Web 设置保存后即时生效，无需重启。
// 空值跳过（与设置页"留空表示不修改"语义一致）。进行中任务持有旧工具实例，不受影响。
func (s *Service) SaveCredentials(appID, accessToken, apiKey, mediaKitAPIKey string) error {
	set := func(key, val string) error {
		if val == "" {
			return nil
		}
		return config.Set(key, val)
	}
	if err := errors.Join(
		set("volc.speech.app_id", appID),
		set("volc.speech.access_token", accessToken),
		set("volc.speech.api_key", apiKey),
		set("volc.mediakit.api_key", mediaKitAPIKey),
	); err != nil {
		return err
	}
	nc := *s.cfg.Load()
	if appID != "" {
		nc.Volc.Speech.AppID = appID
	}
	if accessToken != "" {
		nc.Volc.Speech.AccessToken = accessToken
	}
	if apiKey != "" {
		nc.Volc.Speech.APIKey = apiKey
	}
	if mediaKitAPIKey != "" {
		nc.Volc.MediaKit.APIKey = mediaKitAPIKey
	}
	s.cfg.Store(&nc)
	volcengine.ReRegisterAll(s.reg, nc, nc.DataDir)
	return nil
}

// ReloadVolc 从磁盘配置热应用火山凭证段。配置文件监听（config.Watch）的回调路径：
// 服务运行中另一终端 toolbox config set、手工编辑 config.yaml 的凭证变更即时生效，
// 与 Web 设置保存（SaveCredentials 同步热应用）殊途同归。
// 仅替换 Volc 段：端口与数据目录是启动期属性（监听已绑定、DB 已打开），不跟随文件变更。
func (s *Service) ReloadVolc(disk *config.Config) {
	nc := *s.cfg.Load()
	nc.Volc = disk.Volc
	s.cfg.Store(&nc)
	volcengine.ReRegisterAll(s.reg, nc, nc.DataDir)
}

func (s *Service) Close() error { return nil } // gorm/sqlite 由进程退出回收；预留关闭钩子

// TestSpeechConnection 用音色/凭证连通性检测：构造 TTS 客户端发 1 字合成请求。
// 注意：真实调用会消耗少量合成配额，可接受。
func (s *Service) TestSpeechConnection() (string, bool) {
	cfg := s.cfg.Load()
	cred := volcengine.SpeechCred{
		AppID: cfg.Volc.Speech.AppID, AccessToken: cfg.Volc.Speech.AccessToken, APIKey: cfg.Volc.Speech.APIKey,
	}
	if err := cred.Validate(); err != nil {
		return err.Error(), false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := volcengine.NewTTSClient(cred)
	_, err := client.Synthesize(ctx, volcengine.TTSSynthesizeReq{Text: "测", VoiceType: "zh_female_cancan_mars_bigtts", Format: "mp3"})
	if err != nil {
		return err.Error(), false
	}
	return "连接成功", true
}

// TestMediaKitConnection MediaKit 连通性探测（与人声分离工具同域、同 Bearer 鉴权头）：
// GET 一个必然不存在的任务 ID——404/400 表示鉴权通过（任务不存在属预期）→ 连接成功；
// 401/403 → 凭证无效；网络错误透传错误信息。未配置 apiKey 时直接报未配置，不发起请求。
func (s *Service) TestMediaKitConnection() (string, bool) {
	if s.cfg.Load().Volc.MediaKit.APIKey == "" {
		return "未配置 AI MediaKit API Key：请执行 toolbox config set volc.mediakit.api_key 或在 Web 设置页配置", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		volcengine.MediaKitBaseURL+fmt.Sprintf(volcengine.MediaKitQueryPathFmt, "nonexistent-connectivity-probe"), nil)
	if err != nil {
		return err.Error(), false
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.Load().Volc.MediaKit.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error(), false
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "凭证无效", false
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest:
		return "连接成功", true
	default:
		return fmt.Sprintf("MediaKit 探测异常(HTTP %d)", resp.StatusCode), false
	}
}
