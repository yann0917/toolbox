// Package config 管理 ~/.toolbox/config.yaml：凭证、端口、数据目录。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	v := viper.New()
	v.SetDefault("server.port", 8080)
	v.SetDefault("data_dir", filepath.Join(homeDir(), ".toolbox", "data"))
	v.SetConfigFile(Path())
	v.SetConfigType("yaml")
	// viper 对不存在的 SetConfigFile 返回 *fs.PathError，直接忽略任何读取错误：
	// 仅当文件确实存在时读取失败才报错。
	if err := v.ReadInConfig(); err != nil {
		if _, statErr := os.Stat(Path()); statErr == nil {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
	}
	cfg := &Config{
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
	return cfg, nil
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
