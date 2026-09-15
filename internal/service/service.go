// Package service 组装配置、存储、注册表与任务引擎，是 CLI 与 Web 的唯一共享入口。
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/objectstorage"
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

	// storageMu 守护对象存储客户端的替换（Web 保存存储配置 / 配置文件监听热更新）。
	// 任务提交经 Engine.SetStorageClient 的 getter 取当前客户端，进行中任务不受替换影响。
	storageMu     sync.RWMutex
	storageClient objectstorage.Client
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
	s.rebuildStorageClient(cfg.Storage)
	return s, nil
}

func (s *Service) StartEngine(notify func(task.Event), concurrency int) {
	s.engine = task.New(s.db, s.reg, s.cfg.Load().DataDir, concurrency, notify)
	s.engine.SetStorageClient(s.StorageClient)
}

// StorageClient 当前对象存储客户端（未配置返回 nil），任务引擎经此热取。
// 返回窄接口（Put/PresignGet）：工具只见转存所需的最小能力。
func (s *Service) StorageClient() provider.StorageClient {
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()
	return s.storageClient
}

// storageFull 完整能力视图（探活/生命周期等管理动作用），未配置返回 nil。
func (s *Service) storageFull() objectstorage.Client {
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()
	return s.storageClient
}

// rebuildStorageClient 按存储段配置重建客户端（启动与热更新共用）；配置不完整时置 nil。
func (s *Service) rebuildStorageClient(sc config.StorageConfig) {
	cli, err := objectstorage.New(objectstorage.Config{
		Provider:      sc.Provider,
		Endpoint:      sc.Endpoint,
		Region:        sc.Region,
		Bucket:        sc.Bucket,
		AccessKey:     sc.AccessKey,
		SecretKey:     sc.SecretKey,
		Prefix:        sc.Prefix,
		LifecycleDays: sc.LifecycleDays,
	})
	s.storageMu.Lock()
	s.storageClient = cli
	s.storageMu.Unlock()
	// 构造失败（如 provider 拼写错误）仅在取用时以 ErrNotConfigured 语义暴露，此处置 nil 即可。
	_ = err
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

// ReloadDiskConfig 从磁盘配置热应用运行期可变段：火山凭证 + 对象存储。
// 配置文件监听（config.Watch）的回调路径：服务运行中另一终端 toolbox config set、
// 手工编辑 config.yaml 的变更即时生效，与 Web 设置保存（SaveCredentials/SaveStorage
// 同步热应用）殊途同归。仅替换这两段：端口与数据目录是启动期属性（监听已绑定、
// DB 已打开），不跟随文件变更。
func (s *Service) ReloadDiskConfig(disk *config.Config) {
	nc := *s.cfg.Load()
	nc.Volc = disk.Volc
	nc.Storage = disk.Storage
	s.cfg.Store(&nc)
	volcengine.ReRegisterAll(s.reg, nc, nc.DataDir)
	s.rebuildStorageClient(nc.Storage)
}

// SaveStorage 持久化对象存储段到 config.yaml 并热应用（重建客户端，下一任务即用新通道）。
// 表单语义：文本字段按提交值保存（provider 留空=停用）；secret_key 留空表示不修改。
func (s *Service) SaveStorage(sc config.StorageConfig) error {
	sc.Provider = strings.TrimSpace(sc.Provider)
	switch sc.Provider {
	case "":
	case "tos":
	default:
		return fmt.Errorf("暂不支持该对象存储 provider %q（当前支持 tos，s3 兼容通道规划中）", sc.Provider)
	}
	if sc.Provider != "" {
		missing := []string{}
		for k, v := range map[string]string{
			"endpoint": sc.Endpoint, "region": sc.Region, "bucket": sc.Bucket, "access_key": sc.AccessKey,
		} {
			if strings.TrimSpace(v) == "" {
				missing = append(missing, k)
			}
		}
		// secret 留空且原配置也为空才算缺：留空=沿用已存值。
		if sc.SecretKey == "" && s.cfg.Load().Storage.SecretKey == "" {
			missing = append(missing, "secret_key")
		}
		if len(missing) > 0 {
			return fmt.Errorf("启用对象存储需填写: %s", strings.Join(missing, ", "))
		}
	}

	nc := s.cfg.Load().Storage
	if err := errors.Join(
		config.Set("storage.provider", sc.Provider),
		config.Set("storage.endpoint", strings.TrimSpace(sc.Endpoint)),
		config.Set("storage.region", strings.TrimSpace(sc.Region)),
		config.Set("storage.bucket", strings.TrimSpace(sc.Bucket)),
		config.Set("storage.access_key", strings.TrimSpace(sc.AccessKey)),
		config.Set("storage.prefix", strings.TrimSpace(sc.Prefix)),
		config.Set("storage.lifecycle_days", fmt.Sprint(sc.LifecycleDays)),
	); err != nil {
		return err
	}
	if sc.SecretKey != "" {
		if err := config.Set("storage.secret_key", sc.SecretKey); err != nil {
			return err
		}
		nc.SecretKey = sc.SecretKey
	}
	// 同步热应用内存快照（磁盘权威值由 ReloadDiskConfig 兜底一致）。
	nc.Provider, nc.Endpoint, nc.Region, nc.Bucket = sc.Provider,
		strings.TrimSpace(sc.Endpoint), strings.TrimSpace(sc.Region), strings.TrimSpace(sc.Bucket)
	nc.AccessKey, nc.Prefix, nc.LifecycleDays = strings.TrimSpace(sc.AccessKey),
		strings.TrimSpace(sc.Prefix), sc.LifecycleDays
	snap := *s.cfg.Load()
	snap.Storage = nc
	s.cfg.Store(&snap)
	s.rebuildStorageClient(nc)
	return nil
}

// TestStorageConnection 对象存储探活（HeadBucket）：桶可达且凭证有效即成功。
func (s *Service) TestStorageConnection() (string, bool) {
	cli := s.storageFull()
	if cli == nil {
		return "对象存储未配置", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := cli.Ping(ctx); err != nil {
		return err.Error(), false
	}
	return "连接成功", true
}

// ApplyStorageLifecycle 应用桶生命周期规则（替换本工具前缀规则、保留其他规则）。
// days <= 0 时回落配置中的 lifecycle_days。
func (s *Service) ApplyStorageLifecycle(days int) (string, error) {
	cli := s.storageFull()
	if cli == nil {
		return "", fmt.Errorf("对象存储未配置：请先在设置页填写并保存存储配置")
	}
	if days <= 0 {
		days = s.cfg.Load().Storage.LifecycleDays
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return cli.ApplyLifecycle(ctx, days)
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
