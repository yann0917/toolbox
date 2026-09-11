package volcengine

import (
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
)

// RegisterAll 将火山引擎的全部工具注册进 registry。
// 凭证缺失时 TTS 仍注册（Run 时再报凭证错误）。
func RegisterAll(reg *provider.Registry, cfg config.Config, dataDir string) error {
	cred := SpeechCred{
		AppID:       cfg.Volc.Speech.AppID,
		AccessToken: cfg.Volc.Speech.AccessToken,
		APIKey:      cfg.Volc.Speech.APIKey,
	}
	return reg.Register(NewTTSTool(cred, dataDir))
}
