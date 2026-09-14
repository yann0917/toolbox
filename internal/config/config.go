// Package config 管理 ~/.toolbox/config.yaml：凭证、端口、数据目录。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig
	DataDir string
	Volc    VolcConfig
}

type ServerConfig struct{ Port int }

type VolcConfig struct {
	Speech   SpeechConfig
	MediaKit MediaKitConfig
}

type SpeechConfig struct {
	AppID       string
	AccessToken string
	APIKey      string
}

type MediaKitConfig struct{ APIKey string }

type KV struct{ Key, Value string }

// Dict 命名词典：热词（ASR/妙记）与术语（翻译）的可复用资产，落 config.yaml 的 dicts 段。
type Dict struct {
	Name     string `json:"name"`
	Hotwords string `json:"hotwords,omitempty"`
	Terms    string `json:"terms,omitempty"`
}

type DictEntry struct {
	Hotwords string `mapstructure:"hotwords"`
	Terms    string `mapstructure:"terms"`
}

// Dicts 读取全部命名词典，按名称排序；无 dicts 段时返回空切片。
func Dicts() ([]Dict, error) {
	entries, err := readDicts()
	if err != nil {
		return nil, err
	}
	out := make([]Dict, 0, len(entries))
	for name, e := range entries {
		out = append(out, Dict{Name: name, Hotwords: e.Hotwords, Terms: e.Terms})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// readDicts 注意：UnmarshalKey 把 key 处的「值」解码进目标，因此目标直接是
// map[string]DictEntry，不能再包一层 struct（字段对不上会静默得到空值）。
func readDicts() (map[string]DictEntry, error) {
	v := newFileViper()
	if err := v.ReadInConfig(); err != nil {
		if _, statErr := os.Stat(Path()); statErr == nil {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
	}
	entries := map[string]DictEntry{}
	if err := v.UnmarshalKey("dicts", &entries); err != nil {
		return nil, fmt.Errorf("解析词典失败: %w", err)
	}
	return entries, nil
}

// SetDict 以整段替换的方式 upsert 一个词典（名称含 `.` 时 dotted Set 会拆错层级）。
func SetDict(name string, e DictEntry) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("词典名称不能为空")
	}
	entries, err := readDicts()
	if err != nil {
		return err
	}
	entries[name] = e
	return writeDicts(entries)
}

// RemoveDict 删除词典；不存在时报错。
func RemoveDict(name string) error {
	entries, err := readDicts()
	if err != nil {
		return err
	}
	if _, ok := entries[name]; !ok {
		return fmt.Errorf("词典不存在: %s", name)
	}
	delete(entries, name)
	return writeDicts(entries)
}

func writeDicts(m map[string]DictEntry) error {
	if err := os.MkdirAll(filepath.Dir(Path()), 0o700); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigFile(Path())
	v.SetConfigType("yaml")
	_ = v.ReadInConfig()
	v.Set("dicts", m)
	if err := v.WriteConfigAs(Path()); err != nil {
		return err
	}
	return os.Chmod(Path(), 0o600)
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// Path 返回配置文件路径 ~/.toolbox/config.yaml。
func Path() string {
	return filepath.Join(homeDir(), ".toolbox", "config.yaml")
}

func Load() (*Config, error) {
	v := newFileViper()
	// viper 对不存在的 SetConfigFile 返回 *fs.PathError，直接忽略任何读取错误：
	// 仅当文件确实存在时读取失败才报错。
	if err := v.ReadInConfig(); err != nil {
		if _, statErr := os.Stat(Path()); statErr == nil {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
	}
	return configFromViper(v), nil
}

// newFileViper 绑定 config.yaml 并预置默认值（Load 与 Watch 共用）。
// Set 不走这里：写入路径预置默认值会把它们显式落进用户配置文件。
func newFileViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("server.port", 8080)
	v.SetDefault("data_dir", filepath.Join(homeDir(), ".toolbox", "data"))
	v.SetConfigFile(Path())
	v.SetConfigType("yaml")
	return v
}

func configFromViper(v *viper.Viper) *Config {
	return &Config{
		Server:  ServerConfig{Port: v.GetInt("server.port")},
		DataDir: v.GetString("data_dir"),
		Volc: VolcConfig{
			Speech: SpeechConfig{
				AppID:       v.GetString("volc.speech.app_id"),
				AccessToken: v.GetString("volc.speech.access_token"),
				APIKey:      v.GetString("volc.speech.api_key"),
			},
			MediaKit: MediaKitConfig{APIKey: v.GetString("volc.mediakit.api_key")},
		},
	}
}

// watchDebounce 文件事件防抖：一次保存（尤其编辑器临时文件原子替换）常触发多个事件。
var watchDebounce = 300 * time.Millisecond

// pollInterval 事件兜底轮询间隔。darwin 的 fsnotify 走 kqueue 后端（目录轮扫合成事件），
// 会漏掉"临时文件+改名"式原子替换保存（sed -i / vim 等）；stat 轮询保证编辑器无关的
// 最终一致。CLI config set 与 Web 保存是 truncate 直写，事件可靠、轮询只是冗余兜底。
const pollInterval = 2 * time.Second

// Watch 监听配置文件变更，防抖后以重新读盘的结果回调 onChange。供长驻进程（serve）
// 热加载服务外部的变更：另一终端 toolbox config set、手工编辑 config.yaml；
// Web 设置保存本身同步热应用，不依赖此监听。回调在独立 goroutine 触发，须线程安全。
// 注意：可监听的是配置文件（fsnotify + 低频 stat 兜底）；OS 环境变量无变更通知机制，
// 不在可监听范围。返回的 stop 停止事件与轮询两个触发源。
func Watch(onChange func(*Config)) (stop func(), err error) {
	if err := os.MkdirAll(filepath.Dir(Path()), 0o700); err != nil {
		return nil, err
	}
	// 预建空文件：文件不存在时 viper 建立不了监听（记日志后放弃，后续变更全部丢失）
	if f, ferr := os.OpenFile(Path(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); ferr == nil {
		f.Close()
	} else if !os.IsExist(ferr) {
		return nil, ferr
	}

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	var mu sync.Mutex
	var timer *time.Timer
	stopped := false
	schedule := func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		if timer != nil {
			timer.Stop()
		}
		// 重载一律重新读盘而非读事件方缓存：rename 式保存会漏事件，viper 的
		// 内部配置可能已过期。撕裂读（写一半）时 Load 报错，短重试兜底。
		timer = time.AfterFunc(watchDebounce, func() {
			for i := 0; i < 3; i++ {
				if cfg, lerr := Load(); lerr == nil {
					onChange(cfg)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}

	v := newFileViper()
	v.WatchConfig()
	v.OnConfigChange(func(fsnotify.Event) { schedule() })

	last, serr := os.Stat(Path())
	var lastMod time.Time
	var lastSize int64
	if serr == nil {
		lastMod, lastSize = last.ModTime(), last.Size()
	}
	go func() {
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-t.C:
				if fi, ferr := os.Stat(Path()); ferr == nil {
					if fi.ModTime() != lastMod || fi.Size() != lastSize {
						lastMod, lastSize = fi.ModTime(), fi.Size()
						schedule()
					}
				}
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(stopCh)
			mu.Lock()
			stopped = true
			if timer != nil {
				timer.Stop()
			}
			mu.Unlock()
		})
	}, nil
}

func Set(key, value string) error {
	if err := os.MkdirAll(filepath.Dir(Path()), 0o700); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigFile(Path())
	v.SetConfigType("yaml")
	_ = v.ReadInConfig()
	v.Set(key, value)
	if err := v.WriteConfigAs(Path()); err != nil {
		return err
	}
	return os.Chmod(Path(), 0o600)
}

func List() ([]KV, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	rows := []KV{
		{"server.port", fmt.Sprint(cfg.Server.Port)},
		{"data_dir", cfg.DataDir},
		{"volc.speech.app_id", mask(cfg.Volc.Speech.AppID)},
		{"volc.speech.access_token", mask(cfg.Volc.Speech.AccessToken)},
		{"volc.speech.api_key", mask(cfg.Volc.Speech.APIKey)},
		{"volc.mediakit.api_key", mask(cfg.Volc.MediaKit.APIKey)},
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows, nil
}

// mask 打码敏感值：保留首尾各 1 字符，其余以 * 填充；空值原样返回。
func mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}
