package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yann0917/toolbox/internal/provider/volcengine"
)

type artifactOut struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	DurationMS int64  `json:"duration_ms"`
}

type jsonResult struct {
	TaskID    string         `json:"task_id"`
	Provider  string         `json:"provider"`
	Tool      string         `json:"tool"`
	Status    string         `json:"status"`
	CostMS    int64          `json:"cost_ms"`
	Artifacts []artifactOut  `json:"artifacts"`
	Summary   map[string]any `json:"summary,omitempty"`
	Error     string         `json:"error,omitempty"`
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func eprintf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// absArtifactPath 将产物相对路径解析为基于数据目录的绝对路径（Join 后 Clean）；
// 已是绝对路径则原样返回，保证 JSON 的 artifacts[].path 始终为绝对路径。
func absArtifactPath(dataDir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dataDir, p)
}

// exitCodeFor 将执行错误映射为退出码：2 参数、4 凭证、3 任务失败。
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		return 0
	case strings.Contains(err.Error(), "缺少必填参数"), strings.Contains(err.Error(), "仅支持"),
		strings.Contains(err.Error(), "暂不支持"),     // ASR 音频格式错误（audioFormatOf）/ 播客 format 枚举
		strings.Contains(err.Error(), "speakers"), // 播客音色数量校验（「speakers 需要恰好 2 个」）
		strings.Contains(err.Error(), "对话稿"):      // 播客对话稿解析错误（「对话稿格式错误」）/ 输入互斥文案
		return 2
	case errors.Is(err, volcengine.ErrNoCred), errors.Is(err, volcengine.ErrAuth):
		return 4
	default:
		return 3
	}
}
