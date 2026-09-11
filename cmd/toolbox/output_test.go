package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yann0917/toolbox/internal/provider/volcengine"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{fmt.Errorf("缺少必填参数: text"), 2},
		{fmt.Errorf("%w: xxx", volcengine.ErrNoCred), 4},
		{fmt.Errorf("%w: bad token", volcengine.ErrAuth), 4},
		{errors.New("火山 TTS 错误 内容审核(50000)"), 3},
	}
	for _, c := range cases {
		if got := exitCodeFor(c.err); got != c.want {
			t.Errorf("exitCodeFor(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestAbsArtifactPath(t *testing.T) {
	dataDir := filepath.Join("some", "data")
	if got := absArtifactPath(dataDir, filepath.Join("tts", "u.mp3")); got != filepath.Join("some", "data", "tts", "u.mp3") {
		t.Errorf("relative path not joined with data dir: %q", got)
	}
	if got := absArtifactPath(dataDir, "/abs/out.mp3"); got != "/abs/out.mp3" {
		t.Errorf("absolute path should be kept: %q", got)
	}
	if got := absArtifactPath(dataDir, ""); got != "" {
		t.Errorf("empty path should stay empty: %q", got)
	}
}
