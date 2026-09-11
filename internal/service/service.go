// Package service 组装配置、存储、注册表与任务引擎，是 CLI 与 Web 的唯一共享入口。
package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/provider/volcengine"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/task"
)

type Service struct {
	cfg    *config.Config
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
	return &Service{cfg: cfg, db: db, reg: reg}, nil
}

func (s *Service) StartEngine(notify func(task.Event), concurrency int) {
	s.engine = task.New(s.db, s.reg, s.cfg.DataDir, concurrency, notify)
}

func (s *Service) Engine() *task.Engine {
	if s.engine == nil {
		panic("engine not started: call StartEngine first")
	}
	return s.engine
}

func (s *Service) DB() *store.DB                { return s.db }
func (s *Service) Registry() *provider.Registry { return s.reg }
func (s *Service) Config() *config.Config       { return s.cfg }

func (s *Service) Close() error { return nil } // gorm/sqlite 由进程退出回收；预留关闭钩子

// TestSpeechConnection 用音色/凭证连通性检测：构造 TTS 客户端发 1 字合成请求。
// 注意：真实调用会消耗少量合成配额，可接受。
func (s *Service) TestSpeechConnection() (string, bool) {
	cred := volcengine.SpeechCred{
		AppID: s.cfg.Volc.Speech.AppID, AccessToken: s.cfg.Volc.Speech.AccessToken, APIKey: s.cfg.Volc.Speech.APIKey,
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
