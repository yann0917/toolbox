package volcengine

import (
	"errors"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
)

// RegisterAll 将火山引擎的全部工具注册进 registry。
// 凭证缺失时 TTS/ASR/播客仍注册（Run 时再报凭证错误）。
func RegisterAll(reg *provider.Registry, cfg config.Config, dataDir string) error {
	cred := SpeechCred{
		AppID:       cfg.Volc.Speech.AppID,
		AccessToken: cfg.Volc.Speech.AccessToken,
		APIKey:      cfg.Volc.Speech.APIKey,
	}
	ttsErr := reg.Register(NewTTSTool(cred, dataDir))
	asrErr := reg.Register(NewASRTool(cred, dataDir))
	podErr := reg.Register(NewPodcastTool(cred, dataDir))
	return errors.Join(ttsErr, asrErr, podErr)
}
