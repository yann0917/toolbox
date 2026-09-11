package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// exitCodeFor 将执行错误映射为退出码：2 参数、4 凭证、3 任务失败。
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		return 0
	case strings.Contains(err.Error(), "缺少必填参数"), strings.Contains(err.Error(), "仅支持"):
		return 2
	case errors.Is(err, volcengine.ErrNoCred), errors.Is(err, volcengine.ErrAuth):
		return 4
	default:
		return 3
	}
}
