// Package volcengine 封装火山引擎各服务的客户端与工具实现。
package volcengine

import (
	"errors"
	"fmt"
)

var (
	ErrNoCred = errors.New("凭证未配置")
	ErrAuth   = errors.New("凭证无效")
)

// SpeechCred 语音三件套（TTS/ASR/播客）共用凭证：新版 API Key 或旧版 AppID+AccessToken。
type SpeechCred struct {
	AppID       string
	AccessToken string
	APIKey      string
}

func (c SpeechCred) Validate() error {
	if c.APIKey != "" {
		return nil
	}
	if c.AppID != "" && c.AccessToken != "" {
		return nil
	}
	return fmt.Errorf("%w: 请先执行 toolbox config set volc.speech.app_id <APP ID> 与 volc.speech.access_token <Token>（或设置 volc.speech.api_key）", ErrNoCred)
}
