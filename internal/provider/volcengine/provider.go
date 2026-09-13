package volcengine

import (
	"errors"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
)

// RegisterAll 将火山引擎的全部工具注册进 registry。
// 凭证缺失时 TTS/ASR/播客仍注册（Run 时再报凭证错误）；分离工具用 MediaKit apiKey
// （与语音三件套凭证体系独立），同样缺失仍注册（Run 时再报错）。
func RegisterAll(reg *provider.Registry, cfg config.Config, dataDir string) error {
	cred := SpeechCred{
		AppID:       cfg.Volc.Speech.AppID,
		AccessToken: cfg.Volc.Speech.AccessToken,
		APIKey:      cfg.Volc.Speech.APIKey,
	}
	ttsErr := reg.Register(NewTTSTool(cred, dataDir))
	longErr := reg.Register(NewTTSLongTool(cred, dataDir))
	streamErr := reg.Register(NewTTSStreamTool(cred, dataDir))
	asrErr := reg.Register(NewASRTool(cred, dataDir))
	podErr := reg.Register(NewPodcastTool(cred, dataDir))
	sepErr := reg.Register(NewSeparateTool(cfg.Volc.MediaKit.APIKey, dataDir))
	return errors.Join(ttsErr, longErr, streamErr, asrErr, podErr, sepErr)}
