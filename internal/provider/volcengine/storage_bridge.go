// URL-only 工具的本地文件桥：用户上传的本地文件在任务执行时转存对象存储并换取
// 预签名 GET URL 提交上游，工具侧只看到最终 URL。未配置对象存储时回落「仅公网 URL」
// 并给出设置页指引。数据转存后对上游即用即弃，桶内对象按生命周期规则（默认 3 天）清理。
package volcengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yann0917/toolbox/internal/objectstorage"
	"github.com/yann0917/toolbox/internal/provider"
)

// bridgePresignTTL 预签名 URL 有效期：与桶内对象生命周期（默认 3 天）对齐——
// 闲时任务最长排队 24h，72h 足够覆盖「排队 + 处理 + 重试」；对象到期即随生命周期清理。
const bridgePresignTTL = 72 * time.Hour

// ensureURLInput 解析 URL-only 输入：params[paramKey] 非空直接返回（须为 http(s) 公网地址）；
// 否则取 Files["audio"] 本地文件，经 in.Storage 转存换取签名 URL。两者皆缺或未配置存储时报
// missingErr 指定的中文错误。label 用于进度文案（如「音频」「音视频」）。
func ensureURLInput(ctx context.Context, in provider.TaskInput, paramKey, label, missingErr string, report provider.ProgressReporter) (string, error) {
	if u := strings.TrimSpace(paramString(in.Params, paramKey)); u != "" {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return "", fmt.Errorf("%s URL 须以 http(s):// 开头的公网可访问地址", label)
		}
		return u, nil
	}

	src := in.Files["audio"]
	if src == "" {
		return "", fmt.Errorf("%s", missingErr)
	}
	if in.Storage == nil {
		return "", fmt.Errorf("本地文件需要对象存储中转：请在设置页配置对象存储（推荐火山 TOS），或直接提供公网可访问的 %s URL", label)
	}

	ext := strings.ToLower(filepath.Ext(src))
	if ext == "" {
		return "", fmt.Errorf("无法识别该 %s 文件的扩展名，无法中转（闲时版/极速版依赖 URL 扩展名推断格式）", label)
	}
	f, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("读取 %s 文件失败: %w", label, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("读取 %s 文件信息失败: %w", label, err)
	}

	key := objectstorage.ObjectKey(uuid.NewString() + ext)
	report(3, fmt.Sprintf("正在转存%s到对象存储（%.1f MB）…", label, float64(fi.Size())/1024/1024), nil)
	if err := withUploadProgress(ctx, report, func() error {
		return in.Storage.Put(ctx, key, objectstorage.ContentTypeByExt(key), f, fi.Size())
	}); err != nil {
		return "", err
	}
	signed, err := in.Storage.PresignGet(key, bridgePresignTTL)
	if err != nil {
		return "", err
	}
	report(8, "转存完成，已取得签名 URL", map[string]any{"storage_key": key})
	return signed, nil
}

// withUploadProgress 跑 fn 并每 5s 上报一次心跳（大文件转存期间任务进度不冻结）；
// ctx 取消立即返回取消错误（上传内部随 ctx 中断）。
func withUploadProgress(ctx context.Context, report provider.ProgressReporter, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for i := 1; ; i++ {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return fmt.Errorf("转存已取消: %w", ctx.Err())
		case <-t.C:
			report(min(8, 3+i), fmt.Sprintf("仍在转存到对象存储（%ds）…", i*5), nil)
		}
	}
}
