# Toolbox M1+M2（骨架 + TTS 端到端）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建 toolbox 单二进制骨架（config/store/provider/task/server/CLI），并交付第一个端到端垂直切片：语音合成（TTS）在 CLI 与 Web 上真实可用。

**Architecture:** Go 单 module；cobra CLI 与 gin Web 共享 service 层；goroutine 任务引擎 + SQLite（gorm）状态持久化；provider 抽象隔离火山引擎 SDK 细节；React 19 前端产物 go:embed 内嵌。依据 [设计文档](../specs/2026-09-12-toolbox-design.md)。

**Tech Stack:** Go 1.23+ / gin / gorm(glebarez/sqlite 纯 Go 驱动) / cobra / resty / viper / gorilla/websocket · React 19 / TypeScript / Vite / Tailwind CSS v4 / TanStack Query / Zustand（npm）

## Global Constraints

- Go module 名 `github.com/yann0917/toolbox`；所有 Go 代码在 `cmd/toolbox/`、`internal/`、`main.go`。
- SQLite 驱动必须用 `github.com/glebarez/sqlite`（纯 Go，禁 CGO），保证 `GOOS=windows` 也能交叉编译。
- 凭证与配置只存 `~/.toolbox/config.yaml`（文件权限 0600）；产物默认目录 `~/.toolbox/data`。
- Web 端口默认 8080；REST 前缀 `/api`；WS 端点 `/api/ws`。
- `--json` 机器可读契约（skill `skills/toolbox/references/cli.md`）：stdout 仅输出单个 JSON 对象；进度/告警走 stderr；退出码 0=成功、2=参数错误、3=任务失败、4=凭证缺失或无效。
- 面向用户的文案（CLI 提示、错误消息、Web 文案）一律中文；代码标识符、注释用英文。
- 提交信息用 conventional commits（feat/test/chore/docs…），每个任务至少一次提交。
- Go 测试用标准库 `testing`（不引 testify）；HTTP mock 用 `httptest`。
- TTS 火山接口：`POST https://openspeech.bytedance.com/api/v1/tts`，Authorization 头 `Bearer;<token>`，cluster `volcano_tts`；响应 `{"code":3000,"message":"success","data":"<base64>"}`。
- 语速/音量参数统一为火山 ratio 口径：speed_ratio 0.2~3.0（默认 1.0）、volume_ratio 0.2~3.0（默认 1.0）。
- 长文本（>1000 字）：按句子边界分段（每段 ≤1000 字）逐段合成；仅 `format=mp3` 支持拼接，其他格式返回参数错误。

---

### Task 1: 项目骨架与构建脚本

**Files:**
- Create: `go.mod`, `main.go`, `Makefile`, `.gitignore`（追加）
- Create: `cmd/toolbox/root.go`（最小 cobra root）

**Interfaces:**
- Produces: `main.go` 调用 `cmd/toolbox.Execute()`（Task 10 完善）；Makefile 目标 `make build`、`make test`、`make web`、`make all`。

- [ ] **Step 1: 初始化 go module 与依赖**

```bash
cd /Users/yabo/wwwroot/toolbox
go mod init github.com/yann0917/toolbox
go get github.com/spf13/cobra@latest github.com/spf13/viper@latest
go get gorm.io/gorm@latest github.com/glebarez/sqlite@latest
go get github.com/gin-gonic/gin@latest github.com/go-resty/resty/v2@latest
go get github.com/gorilla/websocket@latest github.com/google/uuid@latest
```

- [ ] **Step 2: 写最小 root 命令**

`cmd/toolbox/root.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "toolbox",
		Short:   "多媒体 AI 工具箱",
		Version: version,
	}
	return root
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(2)
	}
}
```

`main.go`:

```go
package main

import "github.com/yann0917/toolbox/cmd/toolbox"

func main() {
	toolbox.Execute()
}
```

注意：`cmd/toolbox` 与根 `main.go` 同为 `package main` 会冲突——`main.go` 保持 `package main`，`cmd/toolbox` 目录下所有文件用 `package main` 也允许（不同目录是不同包），但为清晰起见 `cmd/toolbox` 使用 `package main` 并由 `main.go` import 会失败（main 不能被 import）。**改为：`main.go` 只写 `package main; import _ "..."` 不可行，正确做法是 `cmd/toolbox` 用 `package cmd`？** 最终定案：`cmd/toolbox/*.go` 为 `package main`，`main.go` 不存在——`go build ./cmd/toolbox` 即产出二进制。删除上面的 `main.go`，root.go 的 `Execute()` 内部自调：

```go
// cmd/toolbox/root.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "toolbox",
		Short:   "多媒体 AI 工具箱",
		Version: version,
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(2)
	}
}
```

- [ ] **Step 3: 写 Makefile**

```makefile
.PHONY: build test web all clean

build:
	go build -o bin/toolbox ./cmd/toolbox

test:
	go test ./...

web:
	cd web && npm ci && npm run build

all: build

clean:
	rm -rf bin
```

`.gitignore` 追加：

```
bin/
web/node_modules/
web/dist/
```

- [ ] **Step 4: 验证构建与命令可用**

Run: `go build ./cmd/toolbox && go run ./cmd/toolbox --version`
Expected: 输出 `toolbox version dev`

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum cmd Makefile .gitignore
git commit -m "chore: Go module 与 cobra 骨架"
```

---

### Task 2: config 包（viper）

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:

```go
func Path() string                       // ~/.toolbox/config.yaml
func Load() (*Config, error)             // 不存在时用默认值；不创建文件
func Set(key, value string) error        // 写入并保存，文件权限 0600
func List() ([]KV, error)                // 密钥打码后按 key 排序返回
type Config struct {
	Server ServerConfig
	DataDir string
	Volc   VolcConfig
}
type ServerConfig struct{ Port int }
type VolcConfig struct {
	Speech   SpeechConfig
	MediaKit MediaKitConfig
}
type SpeechConfig struct{ AppID, AccessToken, APIKey string }
type MediaKitConfig struct{ APIKey string }
type KV struct{ Key, Value string }
```

- [ ] **Step 1: 写失败测试**

`internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	setHome(t, t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Server.Port)
	}
	if !strings.HasSuffix(cfg.DataDir, ".toolbox/data") {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
}

func TestSetAndLoadSecretMasked(t *testing.T) {
	setHome(t, t.TempDir())
	if err := Set("volc.speech.app_id", "123456789"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("perm = %v, want -rwx for 0600", info.Mode().Perm())
	}
	kvs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, kv := range kvs {
		if kv.Key == "volc.speech.app_id" {
			found = true
			if kv.Value != "1******9" && !strings.Contains(kv.Value, "*") {
				t.Errorf("secret not masked: %q", kv.Value)
			}
		}
	}
	if !found {
		t.Error("app_id not listed")
	}
	cfg, _ := Load()
	if cfg.Volc.Speech.AppID != "123456789" {
		t.Errorf("AppID = %q", cfg.Volc.Speech.AppID)
	}
	_ = filepath.Join // keep import
}

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // windows
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/config/ -v`
Expected: FAIL（`undefined: Path/Load/Set/List`）

- [ ] **Step 3: 实现 config.go**

```go
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
	if err := v.ReadInConfig(); err != nil {
		var nf viper.ConfigFileNotFoundError
		if !errorsAs(err, &nf) {
			if _, statErr := os.Stat(Path()); statErr == nil {
				return nil, fmt.Errorf("读取配置失败: %w", err)
			}
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
```

`errorsAs` 用标准库：把上面 `errorsAs(err, &nf)` 换成：

```go
var nf viper.ConfigFileNotFoundError
_ = nf
// viper 对不存在的 SetConfigFile 返回 *fs.PathError，直接忽略任何读取错误：
if err := v.ReadInConfig(); err != nil {
    if _, statErr := os.Stat(Path()); statErr == nil {
        return nil, fmt.Errorf("读取配置失败: %w", err)
    }
}
```

（即实现时直接删掉 ConfigFileNotFoundError 分支，读取错误仅在文件确实存在时报。）

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: config 包（viper 读写 ~/.toolbox/config.yaml）"
```

---

### Task 3: store 包（gorm 模型与仓库）

**Files:**
- Create: `internal/store/store.go`, `internal/store/models.go`, `internal/store/repo.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: 无（仅 gorm）。
- Produces:

```go
func Open(path string) (*DB, error)   // AutoMigrate + MarkRunningAsInterrupted
type DB struct{ gorm *gorm.DB }
type TaskStatus string // pending|running|succeeded|failed|canceled|interrupted
const (StatusPending TaskStatus = "pending"; StatusRunning = "running"; StatusSucceeded = "succeeded"; StatusFailed = "failed"; StatusCanceled = "canceled"; StatusInterrupted = "interrupted")
type Task struct { ID, Provider, Tool string; Status TaskStatus; Params, ProgressNote, Error, Summary string; Progress int; CostMS int64; CreatedAt, UpdatedAt time.Time }
type Artifact struct { ID, TaskID, Kind, Path, Filename, Format string; Size, DurationMS int64; Meta string; CreatedAt time.Time }
func (d *DB) CreateTask(*Task) error
func (d *DB) UpdateTask(*Task) error
func (d *DB) GetTask(id string) (*Task, error)          // 未找到返回 ErrNotFound
func (d *DB) ListTasks(provider string, statuses []TaskStatus, limit, offset int) ([]Task, int64, error)
func (d *DB) DeleteTask(id string) error                // 级联删除 artifacts 记录
func (d *DB) CreateArtifact(*Artifact) error
func (d *DB) ListArtifacts(taskID string) ([]Artifact, error)
func (d *DB) GetArtifact(id string) (*Artifact, error)  // 未找到返回 ErrNotFound
var ErrNotFound = errors.New("record not found")
```

- [ ] **Step 1: 写失败测试**

`internal/store/store_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTaskCRUDAndInterrupted(t *testing.T) {
	db := openTest(t)
	task := &Task{ID: "t1", Provider: "volcengine", Tool: "tts", Status: StatusRunning, Params: "{}"}
	if err := db.CreateTask(task); err != nil {
		t.Fatal(err)
	}
	// 重开库：running 应被标记 interrupted
	db2, err := Open(filepath.Join(t.TempDir(), "same.db"))
	_ = db2 // 无法复用 tempdb；改为直接调 MarkRunningAsInterrupted 语义
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTask(&Task{ID: "t1", Status: StatusRunning}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMarksRunningAsInterrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	db, _ := Open(path)
	_ = db.CreateTask(&Task{ID: "r1", Provider: "p", Tool: "t", Status: StatusRunning})
	_ = db.CreateTask(&Task{ID: "s1", Provider: "p", Tool: "t", Status: StatusSucceeded})
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := db2.GetTask("r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusInterrupted {
		t.Errorf("r1 status = %s, want interrupted", got.Status)
	}
	got, _ = db2.GetTask("s1")
	if got.Status != StatusSucceeded {
		t.Errorf("s1 status = %s, want succeeded", got.Status)
	}
}

func TestListAndDeleteCascade(t *testing.T) {
	db := openTest(t)
	_ = db.CreateTask(&Task{ID: "t1", Provider: "volcengine", Tool: "tts", Status: StatusSucceeded})
	_ = db.CreateArtifact(&Artifact{ID: "a1", TaskID: "t1", Kind: "audio", Path: "x.mp3", CreatedAt: time.Now()})
	items, total, err := db.ListTasks("volcengine", nil, 10, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("list: items=%d total=%d err=%v", len(items), total, err)
	}
	if err := db.DeleteTask("t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTask("t1"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	arts, _ := db.ListArtifacts("t1")
	if len(arts) != 0 {
		t.Errorf("artifacts not cascaded: %d", len(arts))
	}
}
```

（实现时把 `TestTaskCRUDAndInterrupted` 中 db2 相关两行删掉，该测试只留 Create/Get 断言；真正的 interrupted 语义由 `TestOpenMarksRunningAsInterrupted` 覆盖。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/store/ -v`
Expected: FAIL（undefined: Open 等）

- [ ] **Step 3: 实现 models.go / store.go / repo.go**

`internal/store/models.go`:

```go
package store

import "time"

type TaskStatus string

const (
	StatusPending     TaskStatus = "pending"
	StatusRunning     TaskStatus = "running"
	StatusSucceeded   TaskStatus = "succeeded"
	StatusFailed      TaskStatus = "failed"
	StatusCanceled    TaskStatus = "canceled"
	StatusInterrupted TaskStatus = "interrupted"
)

type Task struct {
	ID           string     `gorm:"primaryKey;size:36"`
	Provider     string     `gorm:"size:32;index"`
	Tool         string     `gorm:"size:32;index"`
	Status       TaskStatus `gorm:"size:16;index"`
	Params       string     `gorm:"type:text"`
	Summary      string     `gorm:"type:text"` // 任务完成摘要 JSON（provider.TaskOutput.Summary 序列化）
	Progress     int
	ProgressNote string
	Error        string `gorm:"type:text"`
	CostMS       int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Artifact struct {
	ID         string    `gorm:"primaryKey;size:36"`
	TaskID     string    `gorm:"size:36;index"`
	Kind       string    `gorm:"size:16"` // audio|transcript|dialog|subtitle
	Path       string    // data 目录相对路径
	Filename   string    `gorm:"size:255"`
	Format     string    `gorm:"size:16"`
	Size       int64
	DurationMS int64
	Meta       string    `gorm:"type:text"` // JSON
	CreatedAt  time.Time
}
```

`internal/store/store.go`:

```go
// Package store 提供 SQLite（gorm）持久化：任务与产物。
package store

import (
	"errors"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("record not found")

type DB struct{ gorm *gorm.DB }

func Open(path string) (*DB, error) {
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := g.AutoMigrate(&Task{}, &Artifact{}); err != nil {
		return nil, err
	}
	d := &DB{gorm: g}
	if err := d.markRunningAsInterrupted(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) markRunningAsInterrupted() error {
	return d.gorm.Model(&Task{}).
		Where("status = ?", StatusRunning).
		Update("status", StatusInterrupted).Error
}
```

`internal/store/repo.go`:

```go
package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

func (d *DB) CreateTask(t *Task) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = time.Now()
	return d.gorm.Create(t).Error
}

func (d *DB) UpdateTask(t *Task) error {
	t.UpdatedAt = time.Now()
	return d.gorm.Save(t).Error
}

func (d *DB) GetTask(id string) (*Task, error) {
	var t Task
	if err := d.gorm.First(&t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (d *DB) ListTasks(provider string, statuses []TaskStatus, limit, offset int) ([]Task, int64, error) {
	q := d.gorm.Model(&Task{})
	if provider != "" {
		q = q.Where("provider = ?", provider)
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	var items []Task
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (d *DB) DeleteTask(id string) error {
	return d.gorm.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", id).Delete(&Artifact{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&Task{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (d *DB) CreateArtifact(a *Artifact) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	return d.gorm.Create(a).Error
}

func (d *DB) ListArtifacts(taskID string) ([]Artifact, error) {
	var items []Artifact
	err := d.gorm.Order("created_at ASC").Find(&items, "task_id = ?", taskID).Error
	return items, err
}

func (d *DB) GetArtifact(id string) (*Artifact, error) {
	var a Artifact
	if err := d.gorm.First(&a, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}
```

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat: store 包（任务/产物模型与仓库）"
```

---

### Task 4: provider 抽象与注册表

**Files:**
- Create: `internal/provider/types.go`, `internal/provider/registry.go`
- Test: `internal/provider/registry_test.go`

**Interfaces:**
- Produces:

```go
type ParamType string // string|text|int|float|bool|enum|file
type ParamOption struct{ Value, Label string }
type ParamSpec struct{ Key, Label string; Type ParamType; Required bool; Default any; Options []ParamOption; Placeholder, Group string }
type ToolMeta struct{ Provider, Name, Title, Description, Group string }
type TaskInput struct{ Params map[string]any; Files map[string]string }
type Artifact struct{ Kind, Path, Format string; Size, DurationMS int64; Meta map[string]any }
type TaskOutput struct{ Artifacts []Artifact; Summary map[string]any }
type ProgressReporter func(progress int, note string, detail map[string]any)
type Tool interface{ Meta() ToolMeta; ParamSpecs() []ParamSpec; Run(ctx context.Context, in TaskInput, report ProgressReporter) (TaskOutput, error) }
type Registry struct{ /* ... */ }
func NewRegistry() *Registry
func (r *Registry) Register(t Tool) error        // 重复 (provider,name) 返回错误
func (r *Registry) Get(provider, name string) (Tool, bool)
func (r *Registry) List() []ToolMeta
```

- [ ] **Step 1: 写失败测试**

`internal/provider/registry_test.go`:

```go
package provider

import (
	"context"
	"testing"
)

type fakeTool struct{}

func (fakeTool) Meta() ToolMeta { return ToolMeta{Provider: "p", Name: "t", Title: "T"} }
func (fakeTool) ParamSpecs() []ParamSpec {
	return []ParamSpec{{Key: "text", Label: "文本", Type: ParamText, Required: true}}
}
func (fakeTool) Run(context.Context, TaskInput, ProgressReporter) (TaskOutput, error) {
	return TaskOutput{}, nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeTool{}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(fakeTool{}); err == nil {
		t.Error("duplicate register should fail")
	}
	got, ok := r.Get("p", "t")
	if !ok || got.Meta().Name != "t" {
		t.Fatalf("Get = %v %v", got, ok)
	}
	if len(r.List()) != 1 {
		t.Errorf("List len = %d", len(r.List()))
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/provider/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 types.go 与 registry.go**

`internal/provider/types.go`:

```go
// Package provider 定义平台无关的工具抽象：新平台实现 Tool 并注册即可接入。
package provider

import "context"

type ParamType string

const (
	ParamString ParamType = "string"
	ParamText   ParamType = "text"
	ParamInt    ParamType = "int"
	ParamFloat  ParamType = "float"
	ParamBool   ParamType = "bool"
	ParamEnum   ParamType = "enum"
	ParamFile   ParamType = "file"
)

type ParamOption struct {
	Value string
	Label string
}

type ParamSpec struct {
	Key         string
	Label       string
	Type        ParamType
	Required    bool
	Default     any
	Options     []ParamOption
	Placeholder string
	Group       string
}

type ToolMeta struct {
	Provider    string
	Name        string
	Title       string
	Description string
	Group       string
}

type TaskInput struct {
	Params map[string]any
	Files  map[string]string
}

type Artifact struct {
	Kind       string // audio|transcript|dialog|subtitle
	Path       string // data 目录相对路径
	Format     string
	Size       int64
	DurationMS int64
	Meta       map[string]any
}

type TaskOutput struct {
	Artifacts []Artifact
	Summary   map[string]any
}

// ProgressReporter 上报进度：progress 0-100，note 为中文进度描述，detail 为可选展示数据。
type ProgressReporter func(progress int, note string, detail map[string]any)

type Tool interface {
	Meta() ToolMeta
	ParamSpecs() []ParamSpec
	Run(ctx context.Context, in TaskInput, report ProgressReporter) (TaskOutput, error)
}
```

`internal/provider/registry.go`:

```go
package provider

import "fmt"

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func key(provider, name string) string { return provider + "." + name }

func (r *Registry) Register(t Tool) error {
	m := t.Meta()
	k := key(m.Provider, m.Name)
	if _, exists := r.tools[k]; exists {
		return fmt.Errorf("tool %s already registered", k)
	}
	r.tools[k] = t
	return nil
}

func (r *Registry) Get(provider, name string) (Tool, bool) {
	t, ok := r.tools[key(provider, name)]
	return t, ok
}

func (r *Registry) List() []ToolMeta {
	out := make([]ToolMeta, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Meta())
	}
	return out
}
```

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/provider/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider
git commit -m "feat: provider 工具抽象与注册表"
```

---

### Task 5: task 任务引擎

**Files:**
- Create: `internal/task/engine.go`
- Test: `internal/task/engine_test.go`

**Interfaces:**
- Consumes: `store.DB`（Task 3）、`provider.Tool/Registry`（Task 4）。
- Produces:

```go
type Event struct {
	Type      string  // progress|done|error|canceled
	TaskID    string
	Progress  int
	Note      string
	Error     string
	Artifacts []provider.Artifact
}
func New(db *store.DB, reg *provider.Registry, dataDir string, concurrency int, notify func(Event)) *Engine
func (e *Engine) Submit(providerName, toolName string, params map[string]any, files map[string]string) (string, error)
    // 校验 Required 参数；入库 pending；异步执行。未知工具 -> error。
func (e *Engine) SubmitSync(ctx context.Context, providerName, toolName string, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error)
    // CLI 用：同步执行直到终态；返回最终 task 记录与产物记录。
func (e *Engine) Cancel(id string) error
```

- [ ] **Step 1: 写失败测试**

`internal/task/engine_test.go`:

```go
package task

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type echoTool struct{ fail bool }

func (echoTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{Provider: "fake", Name: "echo", Title: "Echo"}
}
func (echoTool) ParamSpecs() []provider.ParamSpec {
	return []provider.ParamSpec{{Key: "text", Label: "文本", Type: provider.ParamText, Required: true}}
}
func (t echoTool) Run(ctx context.Context, in provider.TaskInput, report provider.ProgressReporter) (provider.TaskOutput, error) {
	report(10, "开始", nil)
	if t.fail {
		return provider.TaskOutput{}, errorFail{}
	}
	select {
	case <-ctx.Done():
		return provider.TaskOutput{}, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	report(100, "完成", nil)
	return provider.TaskOutput{
		Artifacts: []provider.Artifact{{Kind: "audio", Path: "echo/a.mp3", Format: "mp3", Size: 3}},
		Summary:   map[string]any{"text": in.Params["text"]},
	}, nil
}

type errorFail struct{}
func (errorFail) Error() string { return "boom" }

func newTestEngine(t *testing.T, tool provider.Tool, events *[]Event) *Engine {
	t.Helper()
	db, err := OpenStore(t)
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}
	return New(db, reg, t.TempDir(), 2, func(e Event) { *events = append(*events, e) })
}

// OpenStore 在独立目录打开测试库（实现时放 testhelper_test.go）。
func OpenStore(t *testing.T) (*store.DB, error) {
	t.Helper()
	return store.Open(filepath.Join(t.TempDir(), "t.db"))
}

func TestSubmitSyncSuccess(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{}, &events)
	task, arts, err := e.SubmitSync(context.Background(), "fake", "echo",
		map[string]any{"text": "hi"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != store.StatusSucceeded || len(arts) != 1 {
		t.Fatalf("status=%s arts=%d", task.Status, len(arts))
	}
	if arts[0].Kind != "audio" {
		t.Errorf("artifact kind = %s", arts[0].Kind)
	}
	if len(events) == 0 || events[len(events)-1].Type != "done" {
		t.Errorf("last event = %+v", events)
	}
}

func TestSubmitSyncValidation(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{}, &events)
	_, _, err := e.SubmitSync(context.Background(), "fake", "echo", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "text") {
		t.Errorf("err = %v, want missing required param text", err)
	}
	_, _, err = e.SubmitSync(context.Background(), "fake", "nope", map[string]any{}, nil)
	if err == nil {
		t.Error("unknown tool should fail")
	}
}

func TestSubmitSyncFailure(t *testing.T) {
	var events []Event
	e := newTestEngine(t, echoTool{fail: true}, &events)
	task, _, err := e.SubmitSync(context.Background(), "fake", "echo", map[string]any{"text": "x"}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if task.Status != store.StatusFailed || task.Error == "" {
		t.Fatalf("status=%s err=%q", task.Status, task.Error)
	}
}
```

（`strings.Contains` 需要 import "strings"。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/task/ -v`
Expected: FAIL（undefined: New 等）

- [ ] **Step 3: 实现 engine.go**

```go
// Package task 实现任务引擎：参数校验、并发执行、状态落库、事件通知。
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type Event struct {
	Type      string
	TaskID    string
	Progress  int
	Note      string
	Error     string
	Artifacts []provider.Artifact
}

type Engine struct {
	db         *store.DB
	reg        *provider.Registry
	dataDir    string
	sem        chan struct{}
	notify     func(Event)
	mu         sync.Mutex
	cancels    map[string]context.CancelFunc
}

func New(db *store.DB, reg *provider.Registry, dataDir string, concurrency int, notify func(Event)) *Engine {
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Engine{
		db: db, reg: reg, dataDir: dataDir,
		sem:     make(chan struct{}, concurrency),
		notify:  notify,
		cancels: map[string]context.CancelFunc{},
	}
}

func (e *Engine) emit(ev Event) {
	if e.notify != nil {
		e.notify(ev)
	}
}

// validate 检查必填参数；返回错误消息包含缺失参数 key。
func validate(t provider.Tool, params map[string]any) error {
	for _, spec := range t.ParamSpecs() {
		if !spec.Required {
			continue
		}
		v, ok := params[spec.Key]
		if !ok || v == nil || fmt.Sprint(v) == "" {
			return fmt.Errorf("缺少必填参数: %s", spec.Key)
		}
	}
	return nil
}

func (e *Engine) createTask(providerName, toolName string, params map[string]any) (*store.Task, provider.Tool, error) {
	tool, ok := e.reg.Get(providerName, toolName)
	if !ok {
		return nil, nil, fmt.Errorf("未知工具: %s.%s", providerName, toolName)
	}
	if err := validate(tool, params); err != nil {
		return nil, nil, err
	}
	raw, _ := json.Marshal(params)
	t := &store.Task{
		ID: uuid.NewString(), Provider: providerName, Tool: toolName,
		Status: store.StatusPending, Params: string(raw),
	}
	if err := e.db.CreateTask(t); err != nil {
		return nil, nil, err
	}
	return t, tool, nil
}

func (e *Engine) run(ctx context.Context, t *store.Task, tool provider.Tool, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error) {
	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancels[t.ID] = cancel
	e.mu.Unlock()
	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.cancels, t.ID)
		e.mu.Unlock()
	}()

	t.Status = store.StatusRunning
	_ = e.db.UpdateTask(t)
	e.emit(Event{Type: "progress", TaskID: t.ID, Progress: 0, Note: "任务开始"})

	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	report := func(progress int, note string, _ map[string]any) {
		t.Progress = progress
		t.ProgressNote = note
		_ = e.db.UpdateTask(t)
		e.emit(Event{Type: "progress", TaskID: t.ID, Progress: progress, Note: note})
	}

	out, runErr := tool.Run(ctx, provider.TaskInput{Params: params, Files: files}, report)

	var saved []store.Artifact
	for _, a := range out.Artifacts {
		raw, _ := json.Marshal(a.Meta)
		sa := store.Artifact{
			ID: uuid.NewString(), TaskID: t.ID, Kind: a.Kind, Path: a.Path,
			Filename: filepath.Base(a.Path), Format: a.Format,
			Size: a.Size, DurationMS: a.DurationMS, Meta: string(raw),
		}
		if err := e.db.CreateArtifact(&sa); err != nil {
			return t, saved, fmt.Errorf("保存产物失败: %w", err)
		}
		saved = append(saved, sa)
	}

	switch {
	case runErr == nil:
		t.Status = store.StatusSucceeded
		t.Progress = 100
		if out.Summary != nil {
			raw, _ := json.Marshal(out.Summary)
			t.Summary = string(raw)
		}
		e.emit(Event{Type: "done", TaskID: t.ID, Progress: 100, Artifacts: out.Artifacts})
	case errors.Is(runErr, context.Canceled):
		t.Status = store.StatusCanceled
		e.emit(Event{Type: "canceled", TaskID: t.ID})
	default:
		t.Status = store.StatusFailed
		t.Error = runErr.Error()
		e.emit(Event{Type: "error", TaskID: t.ID, Error: runErr.Error()})
	}
	_ = e.db.UpdateTask(t)
	return t, saved, runErr
}

func (e *Engine) Submit(providerName, toolName string, params map[string]any, files map[string]string) (string, error) {
	t, tool, err := e.createTask(providerName, toolName, params)
	if err != nil {
		return "", err
	}
	go func() { _, _, _ = e.run(context.Background(), t, tool, params, files) }()
	return t.ID, nil
}

func (e *Engine) SubmitSync(ctx context.Context, providerName, toolName string, params map[string]any, files map[string]string) (*store.Task, []store.Artifact, error) {
	t, tool, err := e.createTask(providerName, toolName, params)
	if err != nil {
		return nil, nil, err
	}
	return e.run(ctx, t, tool, params, files)
}

func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	cancel, ok := e.cancels[id]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("任务不在运行中: %s", id)
	}
	cancel()
	return nil
}
```

import 补充：`path/filepath`、`errors`。

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/task/ -v`
Expected: PASS（3 个测试）

- [ ] **Step 5: Commit**

```bash
git add internal/task
git commit -m "feat: 任务引擎（并发池/状态机/事件通知）"
```

---

### Task 6: volcengine 凭证与 TTS 客户端

**Files:**
- Create: `internal/provider/volcengine/auth.go`, `internal/provider/volcengine/tts_client.go`
- Test: `internal/provider/volcengine/tts_client_test.go`

**Interfaces:**
- Consumes: `config.SpeechConfig`（Task 2）、resty。
- Produces:

```go
type SpeechCred struct{ AppID, AccessToken, APIKey string }
func (c SpeechCred) Validate() error   // 二选一：APIKey 或 (AppID+AccessToken)，缺失返回退出码语义的 ErrNoCred
var ErrNoCred = errors.New("凭证未配置")
type TTSSynthesizeReq struct {
	Text       string
	VoiceType  string
	Format     string  // mp3|wav|pcm|ogg_opus
	SpeedRatio float64 // 0.2-3.0
	VolumeRatio float64
}
type TTSSynthesizeResp struct{ Audio []byte; DurationMS int64 }
type TTSClient struct{ /* resty + cred */ }
func NewTTSClient(cred SpeechCred) *TTSClient
func (c *TTSClient) Synthesize(ctx context.Context, req TTSSynthesizeReq) (TTSSynthesizeResp, error)
// 错误语义：HTTP 4xx + code!=3000 → 含火山 code/message 的错误；401/鉴权 → ErrAuth
var ErrAuth = errors.New("凭证无效")
```

- [ ] **Step 1: 写失败测试（httptest mock）**

`internal/provider/volcengine/tts_client_test.go`:

```go
package volcengine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMockServer(t *testing.T, status int, body map[string]any, gotHeaders *map[string]string, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := map[string]string{"Authorization": r.Header.Get("Authorization")}
		*gotHeaders = h
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		*gotBody = m
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestSynthesizeSuccess(t *testing.T) {
	audio := []byte("FAKE_MP3_DATA")
	var headers map[string]string
	var body map[string]any
	srv := newMockServer(t, 200, map[string]any{
		"code": 3000, "message": "success",
		"data": base64.StdEncoding.EncodeToString(audio),
		"addition": map[string]any{"duration": "8400"},
	}, &headers, &body)
	defer srv.Close()

	c := NewTTSClientWithBaseURL(SpeechCred{AppID: "app", AccessToken: "tok"}, srv.URL)
	resp, err := c.Synthesize(context.Background(), TTSSynthesizeReq{
		Text: "你好", VoiceType: "v1", Format: "mp3", SpeedRatio: 1.0, VolumeRatio: 1.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Audio) != string(audio) || resp.DurationMS != 8400 {
		t.Fatalf("resp = %+v", resp)
	}
	if headers["Authorization"] != "Bearer;tok" {
		t.Errorf("auth header = %q", headers["Authorization"])
	}
	appMap := body["app"].(map[string]any)
	if appMap["appid"] != "app" || appMap["cluster"] != "volcano_tts" {
		t.Errorf("app = %v", appMap)
	}
}

func TestSynthesizeAuthError(t *testing.T) {
	var h, b map[string]any
	srv := newMockServer(t, 200, map[string]any{
		"code": 3001, "message": "invalid token",
	}, &h, &b)
	defer srv.Close()
	c := NewTTSClientWithBaseURL(SpeechCred{AppID: "a", AccessToken: "bad"}, srv.URL)
	_, err := c.Synthesize(context.Background(), TTSSynthesizeReq{Text: "x", VoiceType: "v", Format: "mp3"})
	if err == nil || !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("err = %v", err)
	}
}

func TestCredValidate(t *testing.T) {
	if err := (SpeechCred{}).Validate(); err == nil {
		t.Error("empty cred should fail")
	}
	if err := (SpeechCred{APIKey: "k"}).Validate(); err != nil {
		t.Errorf("api key only: %v", err)
	}
	if err := (SpeechCred{AppID: "a", AccessToken: "t"}).Validate(); err != nil {
		t.Errorf("appid+token: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/provider/volcengine/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 auth.go 与 tts_client.go**

`internal/provider/volcengine/auth.go`:

```go
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
```

`internal/provider/volcengine/tts_client.go`:

```go
package volcengine

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	ttsBaseURL   = "https://openspeech.bytedance.com"
	ttsCluster   = "volcano_tts"
	ttsCodeOK    = 3000
)

type TTSSynthesizeReq struct {
	Text        string
	VoiceType   string
	Format      string
	SpeedRatio  float64
	VolumeRatio float64
}

type TTSSynthesizeResp struct {
	Audio      []byte
	DurationMS int64
}

type TTSClient struct {
	resty *resty.Client
	cred  SpeechCred
}

func NewTTSClient(cred SpeechCred) *TTSClient {
	return NewTTSClientWithBaseURL(cred, ttsBaseURL)
}

// NewTTSClientWithBaseURL 供测试注入 mock 地址。
func NewTTSClientWithBaseURL(cred SpeechCred, baseURL string) *TTSClient {
	r := resty.New().SetBaseURL(baseURL).
		SetHeader("Content-Type", "application/json").
		SetTimeout(60 * time.Second)
	return &TTSClient{resty: r, cred: cred}
}

type ttsAPIRequest struct {
	App struct {
		AppID   string `json:"appid"`
		Token   string `json:"token"`
		Cluster string `json:"cluster"`
	} `json:"app"`
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		VoiceType   string  `json:"voice_type"`
		Encoding    string  `json:"encoding"`
		SpeedRatio  float64 `json:"speed_ratio"`
		VolumeRatio float64 `json:"volume_ratio"`
	} `json:"audio"`
	Request struct {
		ReqID     string `json:"reqid"`
		Text      string `json:"text"`
		Operation string `json:"operation"`
	} `json:"request"`
}

type ttsAPIResponse struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Data     string `json:"data"`
	Addition struct {
		Duration string `json:"duration"`
	} `json:"addition"`
}

func (c *TTSClient) Synthesize(ctx context.Context, req TTSSynthesizeReq) (TTSSynthesizeResp, error) {
	if req.SpeedRatio == 0 {
		req.SpeedRatio = 1.0
	}
	if req.VolumeRatio == 0 {
		req.VolumeRatio = 1.0
	}
	if req.VoiceType == "" {
		req.VoiceType = "zh_female_cancan_mars_bigtts"
	}

	var apiReq ttsAPIRequest
	apiReq.App.AppID = c.cred.AppID
	apiReq.App.Token = c.cred.AccessToken
	apiReq.App.Cluster = ttsCluster
	apiReq.User.UID = "toolbox"
	apiReq.Audio.VoiceType = req.VoiceType
	apiReq.Audio.Encoding = req.Format
	apiReq.Audio.SpeedRatio = req.SpeedRatio
	apiReq.Audio.VolumeRatio = req.VolumeRatio
	apiReq.Request.ReqID = newRequestID()
	apiReq.Request.Text = req.Text
	apiReq.Request.Operation = "query"

	httpResp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer;"+c.cred.AccessToken).
		SetBody(apiReq).
		Post("/api/v1/tts")
	if err != nil {
		return TTSSynthesizeResp{}, fmt.Errorf("请求火山 TTS 失败: %w", err)
	}

	var apiResp ttsAPIResponse
	if err := resty.Unmarshal(httpResp, &apiResp); err != nil {
		return TTSSynthesizeResp{}, fmt.Errorf("解析火山 TTS 响应失败: %w", err)
	}
	if apiResp.Code != ttsCodeOK {
		if apiResp.Code == 3001 || apiResp.Code == 3005 {
			return TTSSynthesizeResp{}, fmt.Errorf("%w: %s(%d)", ErrAuth, apiResp.Message, apiResp.Code)
		}
		return TTSSynthesizeResp{}, fmt.Errorf("火山 TTS 错误 %s(%d)", apiResp.Message, apiResp.Code)
	}
	audio, err := base64.StdEncoding.DecodeString(apiResp.Data)
	if err != nil {
		return TTSSynthesizeResp{}, fmt.Errorf("音频数据解码失败: %w", err)
	}
	var durationMS int64
	if d, err := strconv.ParseInt(apiResp.Addition.Duration, 10, 64); err == nil {
		durationMS = d
	}
	return TTSSynthesizeResp{Audio: audio, DurationMS: durationMS}, nil
}
```

补充同文件内的 `newRequestID`（或放 auth.go）：

```go
func newRequestID() string { return uuid.NewString() }
```

（import `github.com/google/uuid`。`resty.Unmarshal` 不可用——改为 `json.Unmarshal(httpResp.Body(), &apiResp)`，import `encoding/json`。）

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/provider/volcengine/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/volcengine
git commit -m "feat: 火山语音凭证与 TTS HTTP 客户端"
```

---

### Task 7: TTS Tool（含长文本分段）

**Files:**
- Create: `internal/provider/volcengine/tts_tool.go`, `internal/provider/volcengine/split.go`
- Test: `internal/provider/volcengine/tts_tool_test.go`, `internal/provider/volcengine/split_test.go`

**Interfaces:**
- Consumes: `provider.Tool`（Task 4）、`TTSClient`（Task 6）。
- Produces:

```go
type TTSTool struct{ /* client */ }
func NewTTSTool(cred SpeechCred) *TTSTool
// 实现 provider.Tool：Meta = {Provider:"volcengine", Name:"tts", Title:"语音合成", Group:"语音"}
// ParamSpecs: text(text,required) voice(string,default zh_female_cancan_mars_bigtts)
//             format(enum mp3|wav|pcm|ogg_opus,default mp3) speed_ratio(float,default 1.0) volume_ratio(float,default 1.0)
// Run 产物: Artifact{Kind:"audio", Path:"tts/<reqid>.mp3", Format, DurationMS}
// 长文本: splitText(text, 1000) []string —— 按句号/问号/感叹号/换行切句再贪心组段
// >1000 字且 format != mp3 → "长文本分段合成仅支持 mp3 格式" 参数错误
func splitText(text string, maxLen int) []string
```

- [ ] **Step 1: 写失败测试**

`internal/provider/volcengine/split_test.go`:

```go
package volcengine

import (
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	long := strings.Repeat("这是第一句话。这是第二句话！这是第三句话？", 100) // 1800 字
	segs := splitText(long, 1000)
	if len(segs) < 2 {
		t.Fatalf("segments = %d, want >= 2", len(segs))
	}
	joined := strings.Join(segs, "")
	if len([]rune(joined)) != len([]rune(long)) {
		t.Error("split lost content")
	}
	for i, s := range segs {
		if len([]rune(s)) > 1100 { // 句子本身超长时允许少量溢出
			t.Errorf("seg %d too long: %d", i, len([]rune(s)))
		}
	}
	if got := splitText("短文本", 1000); len(got) != 1 {
		t.Errorf("short text segments = %d", len(got))
	}
}
```

`internal/provider/volcengine/tts_tool_test.go`:

```go
package volcengine

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yann0917/toolbox/internal/provider"
)

func newTTSToolWithMock(t *testing.T, text string, segment int) (*TTSTool, *httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 3000, "message": "success",
			"data":     base64.StdEncoding.EncodeToString([]byte("MP3DATA")),
			"addition": map[string]any{"duration": "1000"},
		})
	}))
	t.Cleanup(srv.Close)
	cred := SpeechCred{AppID: "a", AccessToken: "t"}
	client := NewTTSClientWithBaseURL(cred, srv.URL)
	return &TTSTool{client: client, cred: cred}, srv, text
}

func TestTTSToolRun(t *testing.T) {
	dir := t.TempDir()
	tool, _, _ := newTTSToolWithMock(t, "", 0)
	out, err := tool.Run(context.Background(), provider.TaskInput{
		Params: map[string]any{"text": "你好世界", "voice": "v1", "format": "mp3",
			"speed_ratio": 1.0, "volume_ratio": 1.0},
	}, func(progress int, note string, detail map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Artifacts) != 1 || out.Artifacts[0].Kind != "audio" {
		t.Fatalf("out = %+v", out)
	}
	if !strings.HasPrefix(out.Artifacts[0].Path, "tts/") {
		t.Errorf("artifact path = %s", out.Artifacts[0].Path)
	}
	if _, err := os.Stat(filepath.Join(dir, "nonexist")); os.IsNotExist(err) {
		// 工具本身不写盘（由引擎/服务层负责落盘检查由 Task 8 覆盖）
	}
}

func TestTTSToolRunLongTextWrongFormat(t *testing.T) {
	tool, _, _ := newTTSToolWithMock(t, "", 0)
	long := strings.Repeat("测试句子。", 300) // 1800 字
	_, err := tool.Run(context.Background(), provider.TaskInput{
		Params: map[string]any{"text": long, "format": "wav"},
	}, nopReport)
	if err == nil || !strings.Contains(err.Error(), "mp3") {
		t.Fatalf("err = %v", err)
	}
}

func TestTTSToolMissingText(t *testing.T) {
	tool, _, _ := newTTSToolWithMock(t, "", 0)
	_, err := tool.Run(context.Background(), provider.TaskInput{Params: map[string]any{}}, nopReport)
	if err == nil || !strings.Contains(err.Error(), "text") {
		t.Fatalf("err = %v", err)
	}
}

func nopReport(progress int, note string, detail map[string]any) {}
```

（json import 补 `encoding/json`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/provider/volcengine/ -v`
Expected: FAIL（undefined: TTSTool / splitText）

- [ ] **Step 3: 实现 split.go 与 tts_tool.go**

`internal/provider/volcengine/split.go`:

```go
package volcengine

import "strings"

// splitText 将文本按句末标点切句，再贪心组装为不超过 maxLen（按 rune 计）的段落。
func splitText(text string, maxLen int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxLen {
		return []string{strings.TrimSpace(text)}
	}
	sentEnds := "。！？!?\n"
	var sentences []string
	start := 0
	for i, r := range runes {
		if strings.ContainsRune(sentEnds, r) {
			sentences = append(sentences, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		sentences = append(sentences, string(runes[start:]))
	}

	var segs []string
	var cur strings.Builder
	curLen := 0
	for _, s := range sentences {
		sLen := len([]rune(s))
		if sLen > maxLen {
			if cur.Len() > 0 {
				segs = append(segs, cur.String())
				cur.Reset()
				curLen = 0
			}
			segs = append(segs, s) // 单句超长：整句独立成段
			continue
		}
		if curLen+sLen > maxLen {
			segs = append(segs, cur.String())
			cur.Reset()
			curLen = 0
		}
		cur.WriteString(s)
		curLen += sLen
	}
	if cur.Len() > 0 {
		segs = append(segs, cur.String())
	}
	return segs
}
```

`internal/provider/volcengine/tts_tool.go`:

```go
package volcengine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/yann0917/toolbox/internal/provider"
)

const longTextThreshold = 1000

// TTSTool 语音合成工具（火山 TTS HTTP V1）。
type TTSTool struct {
	client *TTSClient
	cred   SpeechCred
	outDir string // 产物写入目录（默认 ~/.toolbox/data，由构造方注入）
}

func NewTTSTool(cred SpeechCred, outDir string) *TTSTool {
	return &TTSTool{client: NewTTSClient(cred), cred: cred, outDir: outDir}
}

func (t *TTSTool) Meta() provider.ToolMeta {
	return provider.ToolMeta{
		Provider:    "volcengine",
		Name:        "tts",
		Title:       "语音合成",
		Description: "将文本合成为语音，支持多音色、语速音量调节与长文本分段",
		Group:       "语音",
	}
}

func (t *TTSTool) ParamSpecs() []provider.ParamSpec {
	return []provider.ParamSpec{
		{Key: "text", Label: "文本", Type: provider.ParamText, Required: true,
			Placeholder: "输入要合成的文本", Group: "内容"},
		{Key: "voice", Label: "音色", Type: provider.ParamString,
			Default: "zh_female_cancan_mars_bigtts", Group: "参数",
			Placeholder: "音色 ID，用 toolbox voices list 查询"},
		{Key: "format", Label: "音频格式", Type: provider.ParamEnum,
			Default: "mp3", Group: "参数",
			Options: []provider.ParamOption{
				{Value: "mp3", Label: "MP3"}, {Value: "wav", Label: "WAV"},
				{Value: "pcm", Label: "PCM"}, {Value: "ogg_opus", Label: "OGG Opus"},
			}},
		{Key: "speed_ratio", Label: "语速 (0.2-3.0)", Type: provider.ParamFloat, Default: 1.0, Group: "参数"},
		{Key: "volume_ratio", Label: "音量 (0.2-3.0)", Type: provider.ParamFloat, Default: 1.0, Group: "参数"},
	}
}

func (t *TTSTool) Run(ctx context.Context, in provider.TaskInput, report provider.ProgressReporter) (provider.TaskOutput, error) {
	if err := t.cred.Validate(); err != nil {
		return provider.TaskOutput{}, err
	}
	text, _ := in.Params["text"].(string)
	if utf8.RuneCountInString(text) == 0 {
		return provider.TaskOutput{}, fmt.Errorf("缺少必填参数: text")
	}
	format, _ := in.Params["format"].(string)
	if format == "" {
		format = "mp3"
	}
	voice, _ := in.Params["voice"].(string)
	speed, volume := toFloat(in.Params["speed_ratio"], 1.0), toFloat(in.Params["volume_ratio"], 1.0)

	segments := splitText(text, longTextThreshold)
	if len(segments) > 1 && format != "mp3" {
		return provider.TaskOutput{}, fmt.Errorf("长文本分段合成仅支持 mp3 格式（当前 %s），请改用 mp3 或缩短文本", format)
	}

	var audio bytes.Buffer
	var durationMS int64
	for i, seg := range segments {
		report(100*(i)/len(segments), fmt.Sprintf("正在合成第 %d/%d 段", i+1, len(segments)), nil)
		resp, err := t.client.Synthesize(ctx, TTSSynthesizeReq{
			Text: seg, VoiceType: voice, Format: format,
			SpeedRatio: speed, VolumeRatio: volume,
		})
		if err != nil {
			return provider.TaskOutput{}, err
		}
		audio.Write(resp.Audio)
		durationMS += resp.DurationMS
	}
	report(90, "保存音频文件", nil)

	reqID := uuid.NewString()
	relPath := filepath.Join("tts", reqID+"."+extOf(format))
	// _out 参数（CLI --out）重定向产物路径；不进 ParamSpecs，属机器约定。
	if outParam, ok := in.Params["_out"].(string); ok && outParam != "" {
		relPath = outParam
	}
	absPath := relPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(t.outDir, relPath)
		relPath = filepath.Rel(t.outDir, absPath)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return provider.TaskOutput{}, fmt.Errorf("创建产物目录失败: %w", err)
	}
	if err := os.WriteFile(absPath, audio.Bytes(), 0o644); err != nil {
		return provider.TaskOutput{}, fmt.Errorf("写入音频文件失败: %w", err)
	}

	return provider.TaskOutput{
		Artifacts: []provider.Artifact{{
			Kind: "audio", Path: relPath, Format: format,
			Size: int64(audio.Len()), DurationMS: durationMS,
		}},
		Summary: map[string]any{
			"char_count":  utf8.RuneCountInString(text),
			"segment_num": len(segments),
		},
	}, nil
}

func extOf(format string) string {
	if format == "ogg_opus" {
		return "ogg"
	}
	return format
}

func toFloat(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		var f float64
		if _, err := fmt.Sscanf(x, "%g", &f); err == nil {
			return f
		}
	}
	return def
}
```

- [ ] **Step 4: 运行测试通过（更新 Task 6 的 mock 测试中 NewTTSTool 构造签名）**

Run: `go test ./internal/provider/volcengine/ -v`
Expected: PASS（注意 `tts_tool_test.go` 中 `&TTSTool{client: client, cred: cred}` 补 `outDir: t.TempDir()`）

- [ ] **Step 5: Commit**

```bash
git add internal/provider/volcengine
git commit -m "feat: TTS Tool（长文本分段、产物落盘）"
```

---

### Task 8: volcesettings 注册与 service 层

**Files:**
- Create: `internal/service/service.go`
- Test: `internal/service/service_test.go`
- Create: `internal/provider/volcengine/provider.go`（Register 工厂）

**Interfaces:**
- Consumes: config/store/provider/task（Task 2-5）、TTSTool（Task 7）。
- Produces:

```go
// internal/provider/volcengine/provider.go
func RegisterAll(reg *provider.Registry, cfg config.Config) error
    // 构造 TTSTool 并注册；凭证缺失时 TTS 仍注册（Run 时再报凭证错误）
// internal/service/service.go
type Service struct{ /* db, engine, cfg, reg */ }
func New(cfg *config.Config) (*Service, error)  // Open DB(<data_dir>/toolbox.db) + Registry + Engine
func (s *Service) Engine() *task.Engine
func (s *Service) DB() *store.DB
func (s *Service) Registry() *provider.Registry
func (s *Service) Close() error
```

- [ ] **Step 1: 写失败测试**

`internal/service/service_test.go`:

```go
package service

import (
	"path/filepath"
	"testing"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/volcengine" // 占位：实际 import 路径见下方说明
)

// 注意：config 需要支持测试注入 data_dir —— 在 config.Config 增加
// `RuntimeHome string`（仅测试用，空则用 ~/.toolbox）由 setHome 实现，见 Task 8 Step 3。

func TestNewRegistersTools(t *testing.T) {
	svc := newTestService(t)
	metas := svc.Registry().List()
	if len(metas) != 1 {
		t.Fatalf("registered tools = %d, want 1", len(metas))
	}
	if metas[0].Name != "tts" {
		t.Errorf("tool = %s", metas[0].Name)
	}
	_, ok := svc.Registry().Get("volcengine", "tts")
	if !ok {
		t.Error("volcengine.tts not found")
	}
}

func TestSubmitUnknownTool(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Engine().SubmitSync(t.Context(), "volcengine", "asr", map[string]any{}, nil)
	if err == nil {
		t.Error("unknown tool should fail")
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

var _ = provider.ParamString
var _ = volcengine.ErrNoCred
```

说明：`NewWithHome(home string)` 是 `Service` 的测试构造入口——`New(cfg)` 内部调用它。实现时：`func New(cfg *config.Config) (*Service, error)` 用 `cfg.DataDir` 作根；`func NewWithHome(home string) (*Service, error)` 将 dataDir 设为 `<home>/data`、DB 路径 `<home>/data/toolbox.db`。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/service/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 provider.go 与 service.go**

`internal/provider/volcengine/provider.go`:

```go
package volcengine

import (
	"path/filepath"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
)

// RegisterAll 将火山引擎的全部工具注册进 registry。
func RegisterAll(reg *provider.Registry, cfg config.Config, dataDir string) error {
	cred := SpeechCred{
		AppID:       cfg.Volc.Speech.AppID,
		AccessToken: cfg.Volc.Speech.AccessToken,
		APIKey:      cfg.Volc.Speech.APIKey,
	}
	return reg.Register(NewTTSTool(cred, dataDir))
}

func dataSubDir(dataDir, sub string) string { return filepath.Join(dataDir, sub) }
```

（`dataSubDir` 若未使用则删除——保持零未用代码。）

`internal/service/service.go`:

```go
// Package service 组装配置、存储、注册表与任务引擎，是 CLI 与 Web 的唯一共享入口。
package service

import (
	"fmt"
	"path/filepath"

	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/provider/volcengine"
	"github.com/yann0917/toolbox/internal/store"
	"github.com/yann0917/toolbox/internal/task"
)

type Service struct {
	cfg  *config.Config
	db   *store.DB
	reg  *provider.Registry
	engine *task.Engine
}

func New(cfg *config.Config) (*Service, error) {
	return newWithRoot(filepath.Dir(cfg.DataDir), cfg)
}

// NewWithHome 以 home 为 ~/.toolbox 根的测试构造。
func NewWithHome(home string) (*Service, error) {
	cfg, err := loadFor(home)
	if err != nil {
		return nil, err
	}
	return newWithRoot(home, cfg)
}

func loadFor(home string) (*config.Config, error) {
	// 测试场景：直接构造默认配置，DataDir 指向 home/data
	return &config.Config{
		Server:  config.ServerConfig{Port: 0},
		DataDir: filepath.Join(home, "data"),
	}, nil
}

func newWithRoot(home string, cfg *config.Config) (*Service, error) {
	dataDir := cfg.DataDir
	db, err := store.Open(filepath.Join(dataDir, "toolbox.db"))
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	reg := provider.NewRegistry()
	if err := volcengine.RegisterAll(reg, *cfg, dataDir); err != nil {
		return nil, err
	}
	return &Service{cfg: cfg, db: db, reg: reg}, nil
}

func (s *Service) StartEngine(notify func(task.Event), concurrency int) {
	s.engine = task.New(s.db, s.reg, s.cfg.DataDir, concurrency, notify)
}

func (s *Service) Engine() *task.Engine {
	if s.engine == nil {
		panic("engine not started: call StartEngine first")
	}
	return s.engine
}

func (s *Service) DB() *store.DB      { return s.db }
func (s *Service) Registry() *provider.Registry { return s.reg }
func (s *Service) Config() *config.Config { return s.cfg }

func (s *Service) Close() error { return nil } // gorm/sqlite 由进程退出回收；预留关闭钩子
```

CLI 同步用法（Task 10 会用）：`svc.StartEngine(nil, 1)` 后 `svc.Engine().SubmitSync(...)`。

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service internal/provider/volcengine/provider.go
git commit -m "feat: service 组装层与火山工具注册"
```

---

### Task 9: server：REST + WebSocket + settings

**Files:**
- Create: `internal/server/server.go`, `internal/server/routes.go`, `internal/server/hub.go`, `internal/server/dto.go`
- Test: `internal/server/routes_test.go`, `internal/server/hub_test.go`

**Interfaces:**
- Consumes: service（Task 8）。
- Produces:

```go
func New(svc *service.Service) *Server
func (s *Server) Handler() http.Handler        // 全部路由
func (s *Server) Hub() *Hub                    // 广播 task 事件
// 路由:
// GET  /api/health
// GET  /api/tools
// POST /api/tasks            {provider,tool,params} -> {task_id}
// GET  /api/tasks?provider=&status=&page=&size=
// GET  /api/tasks/:id        -> task + artifacts
// DELETE /api/tasks/:id
// POST /api/tasks/:id/cancel
// GET  /api/artifacts/:id/stream   (Range 支持)
// GET  /api/artifacts/:id/download
// GET  /api/settings  -> {volc:{speech:{app_id,has_access_token,api_key},mediakit:{has_api_key}}}
// PUT  /api/settings  {app_id?,access_token?,api_key?,mediakit_api_key?}  // 空串不改
// POST /api/settings/test-connection -> {ok, message}
// GET  /api/voices  -> {voices:[{id,gender,category}]}
// GET  /api/ws      -> WebSocket；连接即发 {"type":"task.snapshot",...}，之后广播事件
```

- [ ] **Step 1: 写失败测试（路由 + hub 广播）**

`internal/server/routes_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yann0917/toolbox/internal/service"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	svc, err := service.NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	s := New(svc)
	svc.StartEngine(s.Hub().Notify, 2)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func TestHealth(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("body = %v", body)
	}
}

func TestToolsAndTaskSubmit(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, _ := http.Get(ts.URL + "/api/tools")
	var tools []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&tools)
	if len(tools) != 1 || tools[0]["name"] != "tts" {
		t.Fatalf("tools = %v", tools)
	}

	body := `{"provider":"volcengine","tool":"tts","params":{"text":"缺凭证也入库","format":"mp3"}}`
	resp2, err := http.Post(ts.URL+"/api/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("submit status = %d", resp2.StatusCode)
	}
	var created map[string]string
	_ = json.NewDecoder(resp2.Body).Decode(&created)
	if created["task_id"] == "" {
		t.Errorf("created = %v", created)
	}

	// 任务列表
	resp3, _ := http.Get(ts.URL + "/api/tasks")
	var list map[string]any
	_ = json.NewDecoder(resp3.Body).Decode(&list)
	if list["total"].(float64) != 1 {
		t.Errorf("list = %v", list)
	}
}
```

`internal/server/hub_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yann0917/toolbox/internal/task"
)

func TestHubBroadcast(t *testing.T) {
	ts, s := newTestServer(t)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	s.Hub().Notify(task.Event{Type: "progress", TaskID: "x", Progress: 42, Note: "测试"})

	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), `"progress"`) || !strings.Contains(string(msg), `"42"`) {
		t.Errorf("msg = %s", msg)
	}
}
```

注意：hub_test 建连即收到 snapshot（若有未完成任务）——断言用 `Contains` 而非精确匹配可兼容。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/server/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 hub.go / dto.go / server.go / routes.go**

`internal/server/hub.go`:

```go
package server

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yann0917/toolbox/internal/task"
)

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub 维护 WS 客户端并广播任务事件；Notify 作为 task.Engine 的回调。
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*client]struct{}{}}
}

func (h *Hub) Notify(ev task.Event) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.broadcast(raw)
}

func (h *Hub) broadcast(raw []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- raw:
		default: // 慢客户端直接丢弃，避免阻塞
		}
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
}

func (h *Hub) writePump(c *client) {
	for raw := range c.send {
		_ = c.conn.WriteMessage(websocket.TextMessage, raw)
	}
}

func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request, snapshotJSON func() []byte) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.register(c)
	go h.writePump(c)
	// 连接建立即补发非终态任务快照，防漏消息
	if snapshotJSON != nil {
		if snap := snapshotJSON(); snap != nil {
			c.send <- snap
		}
	}
	// 读泵：仅处理关闭
	go func() {
		defer func() {
			h.unregister(c)
			_ = conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
```

`internal/server/dto.go`:

```go
package server

import (
	"encoding/json"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type taskDTO struct {
	ID       string           `json:"id"`
	Provider string           `json:"provider"`
	Tool     string           `json:"tool"`
	Status   string           `json:"status"`
	Progress int              `json:"progress"`
	Note     string           `json:"progress_note"`
	Error    string           `json:"error,omitempty"`
	CostMS   int64            `json:"cost_ms"`
	Params   json.RawMessage  `json:"params"`
	CreatedAt string          `json:"created_at"`
}

type artifactDTO struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Filename   string          `json:"filename"`
	Format     string          `json:"format"`
	Size       int64           `json:"size"`
	DurationMS int64           `json:"duration_ms"`
	Meta       json.RawMessage `json:"meta,omitempty"`
}

func toTaskDTO(t store.Task) taskDTO {
	return taskDTO{
		ID: t.ID, Provider: t.Provider, Tool: t.Tool, Status: string(t.Status),
		Progress: t.Progress, Note: t.ProgressNote, Error: t.Error,
		CostMS: t.CostMS, Params: json.RawMessage(t.Params), CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toArtifactDTO(a store.Artifact) artifactDTO {
	return artifactDTO{
		ID: a.ID, Kind: a.Kind, Filename: a.Filename, Format: a.Format,
		Size: a.Size, DurationMS: a.DurationMS, Meta: json.RawMessage(a.Meta),
	}
}

type toolDTO struct {
	Meta       provider.ToolMeta  `json:"meta"`
	ParamSpecs []provider.ParamSpec `json:"param_specs"`
}
```

`internal/server/server.go`:

```go
// Package server 提供 gin HTTP 服务与 WebSocket hub。
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yann0917/toolbox/internal/service"
	"github.com/yann0917/toolbox/internal/store"
)

type Server struct {
	svc *service.Service
	hub *Hub
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc, hub: NewHub()}
}

func (s *Server) Hub() *Hub { return s.hub }

// snapshotJSON 返回非终态任务快照消息。
func (s *Server) snapshotJSON() []byte {
	items, _, err := s.svc.DB().ListTasks("", []store.TaskStatus{store.StatusPending, store.StatusRunning}, 100, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	dtos := make([]taskDTO, 0, len(items))
	for _, t := range items {
		dtos = append(dtos, toTaskDTO(t))
	}
	raw, _ := json.Marshal(map[string]any{"type": "task.snapshot", "tasks": dtos})
	return raw
}
```

`internal/server/routes.go`:

```go
package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/store"
)

func (s *Server) Handler() http.Handler {
	r := gin.Default()
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		api.GET("/tools", s.listTools)
		api.POST("/tasks", s.createTask)
		api.GET("/tasks", s.listTasks)
		api.GET("/tasks/:id", s.getTask)
		api.DELETE("/tasks/:id", s.deleteTask)
		api.POST("/tasks/:id/cancel", s.cancelTask)
		api.GET("/artifacts/:id/stream", s.streamArtifact)
		api.GET("/artifacts/:id/download", s.downloadArtifact)
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.putSettings)
		api.POST("/settings/test-connection", s.testConnection)
		api.GET("/voices", s.listVoices)
		api.GET("/ws", func(c *gin.Context) { s.hub.serveWS(c.Writer, c.Request, s.snapshotJSON) })
	}
	return r
}

func (s *Server) listTools(c *gin.Context) {
	out := []toolDTO{}
	for _, t := range s.svc.Registry().List() {
		tool, _ := s.svc.Registry().Get(t.Provider, t.Name)
		out = append(out, toolDTO{Meta: t, ParamSpecs: tool.ParamSpecs()})
	}
	c.JSON(200, out)
}

type createTaskReq struct {
	Provider string         `json:"provider"`
	Tool     string         `json:"tool"`
	Params   map[string]any `json:"params"`
}

func (s *Server) createTask(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Provider == "" || req.Tool == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：provider/tool 必填"})
		return
	}
	id, err := s.svc.Engine().Submit(req.Provider, req.Tool, req.Params, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"task_id": id})
}

func (s *Server) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	items, total, err := s.svc.DB().ListTasks(c.Query("provider"), nil, size, (page-1)*size)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	dtos := make([]taskDTO, 0, len(items))
	for _, t := range items {
		dtos = append(dtos, toTaskDTO(t))
	}
	c.JSON(200, gin.H{"items": dtos, "total": total})
}

func (s *Server) getTask(c *gin.Context) {
	t, err := s.svc.DB().GetTask(c.Param("id"))
	if err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}
	arts, err := s.svc.DB().ListArtifacts(t.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	adtos := make([]artifactDTO, 0, len(arts))
	for _, a := range arts {
		adtos = append(adtos, toArtifactDTO(a))
	}
	c.JSON(200, gin.H{"task": toTaskDTO(*t), "artifacts": adtos})
}

func (s *Server) deleteTask(c *gin.Context) {
	if err := s.svc.DB().DeleteTask(c.Param("id")); err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) cancelTask(c *gin.Context) {
	if err := s.svc.Engine().Cancel(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// artifactAbsPath 将产物相对路径解析到 data 目录下，防止路径穿越。
func (s *Server) artifactAbsPath(rel string) (string, error) {
	abs := filepath.Join(s.svc.Config().DataDir, rel)
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}

func (s *Server) streamArtifact(c *gin.Context) {
	a, err := s.svc.DB().GetArtifact(c.Param("id"))
	if err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "产物不存在"})
		return
	}
	abs, err := s.artifactAbsPath(a.Path)
	if err != nil {
		c.JSON(404, gin.H{"error": "产物文件缺失"})
		return
	}
	c.Header("Accept-Ranges", "bytes")
	http.ServeFile(c.Writer, c.Request, abs)
}

func (s *Server) downloadArtifact(c *gin.Context) {
	a, err := s.svc.DB().GetArtifact(c.Param("id"))
	if err == store.ErrNotFound {
		c.JSON(404, gin.H{"error": "产物不存在"})
		return
	}
	abs, err := s.artifactAbsPath(a.Path)
	if err != nil {
		c.JSON(404, gin.H{"error": "产物文件缺失"})
		return
	}
	c.FileAttachment(abs, a.Filename)
}

func (s *Server) getSettings(c *gin.Context) {
	cfg := s.svc.Config()
	c.JSON(200, gin.H{
		"volc": gin.H{
			"speech": gin.H{
				"app_id":           cfg.Volc.Speech.AppID,
				"has_access_token": cfg.Volc.Speech.AccessToken != "",
				"api_key":          cfg.Volc.Speech.APIKey,
			},
			"mediakit": gin.H{"has_api_key": cfg.Volc.MediaKit.APIKey != ""},
		},
		"data_dir": cfg.DataDir,
	})
}

type putSettingsReq struct {
	AppID          string `json:"app_id"`
	AccessToken    string `json:"access_token"`
	APIKey         string `json:"api_key"`
	MediaKitAPIKey string `json:"mediakit_api_key"`
}

func (s *Server) putSettings(c *gin.Context) {
	var req putSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	setIfNotEmpty := func(key, val string) {
		if val != "" {
			_ = config.Set(key, val)
		}
	}
	setIfNotEmpty("volc.speech.app_id", req.AppID)
	setIfNotEmpty("volc.speech.access_token", req.AccessToken)
	setIfNotEmpty("volc.speech.api_key", req.APIKey)
	setIfNotEmpty("volc.mediakit.api_key", req.MediaKitAPIKey)
	c.JSON(200, gin.H{"ok": true, "note": "凭证已保存，重启 Web 服务后生效"})
}

func (s *Server) testConnection(c *gin.Context) {
	msg, ok := s.svc.TestSpeechConnection()
	c.JSON(200, gin.H{"ok": ok, "message": msg})
}

func (s *Server) listVoices(c *gin.Context) {
	c.JSON(200, gin.H{"voices": BuiltinVoices()})
}
```

`internal/server/voices.go`（同任务创建）：`BuiltinVoices()` 返回 `[]map[string]string`（字段 `id/gender/category`），预置 10 个常用大模型音色——实现时从火山官方音色列表文档取真实 ID（如 `zh_female_cancan_mars_bigtts` 灿灿/女/通用、`zh_male_dayixiansheng_v2_saturn_bigtts` 大一先生/男/通用 等），后续里程碑接入 `voices list` 在线接口后替换。

service 侧补充 `TestSpeechConnection`（加到 `internal/service/service.go`）：

```go
// TestSpeechConnection 用音色/凭证连通性检测：构造 TTS 客户端发 1 字合成请求。
func (s *Service) TestSpeechConnection() (string, bool) {
	cred := volcengine.SpeechCred{
		AppID: s.cfg.Volc.Speech.AppID, AccessToken: s.cfg.Volc.Speech.AccessToken, APIKey: s.cfg.Volc.Speech.APIKey,
	}
	if err := cred.Validate(); err != nil {
		return err.Error(), false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := volcengine.NewTTSClient(cred)
	_, err := client.Synthesize(ctx, volcengine.TTSSynthesizeReq{Text: "测", VoiceType: "zh_female_cancan_mars_bigtts", Format: "mp3"})
	if err != nil {
		return err.Error(), false
	}
	return "连接成功", true
}
```

（import `context`、`time`、`volcengine`。注意 M2 阶段真实调用消耗少量配额，可接受；文档注明。）

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/server/ ./internal/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server internal/service
git commit -m "feat: gin 路由、WS hub、settings 与音色接口"
```

---

### Task 10: CLI 命令（root/config/voices/tts/serve 骨架）

**Files:**
- Create: `cmd/toolbox/config.go`, `cmd/toolbox/voices.go`, `cmd/toolbox/tts.go`, `cmd/toolbox/output.go`
- Modify: `cmd/toolbox/root.go`
- Test: `cmd/toolbox/output_test.go`

**Interfaces:**
- Consumes: service（Task 8）、config（Task 2）。
- Produces: `toolbox config set/list`、`toolbox voices list`、`toolbox tts ...`、`toolbox serve`（占位到 Task 14）；共享 `runToolSync()`：

```go
// output.go
type jsonResult struct {
	TaskID    string          `json:"task_id"`
	Provider  string          `json:"provider"`
	Tool      string          `json:"tool"`
	Status    string          `json:"status"`
	CostMS    int64           `json:"cost_ms"`
	Artifacts []artifactOut   `json:"artifacts"`
	Summary   map[string]any  `json:"summary,omitempty"`
	Error     string          `json:"error,omitempty"`
}
type artifactOut struct { Kind, Path, Format string; Size, DurationMS int64 }
func exitCodeFor(err error) int  // ErrNoCred*->4; 参数->2; 其他->3
func printJSON(v any)            // stdout 单行 JSON
func eprintf(format string, a ...any) // stderr
```

- [ ] **Step 1: 写失败测试**

`cmd/toolbox/output_test.go`:

```go
package main

import (
	"errors"
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
```

（补 `import "fmt"`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./cmd/toolbox/ -v`
Expected: FAIL（undefined: exitCodeFor）

- [ ] **Step 3: 实现 output.go 与各命令**

`cmd/toolbox/output.go`:

```go
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
```

`cmd/toolbox/config.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "查看与设置凭证及配置"}
	cmd.AddCommand(newConfigSetCmd(), newConfigListCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "设置配置项（volc.speech.app_id / volc.speech.access_token / volc.speech.api_key / volc.mediakit.api_key / server.port）",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if err := config.Set(args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "已保存 %s\n", args[0])
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "查看配置（密钥打码）",
		RunE: func(c *cobra.Command, args []string) error {
			rows, err := config.List()
			if err != nil {
				return err
			}
			for _, kv := range rows {
				fmt.Printf("%-28s %s\n", kv.Key, kv.Value)
			}
			return nil
		},
	}
}
```

`cmd/toolbox/voices.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/server"
)

func newVoicesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "voices", Short: "音色查询"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出内置可用音色",
		RunE: func(c *cobra.Command, args []string) error {
			jsonOut, _ := c.Flags().GetBool("json")
			voices := server.BuiltinVoices()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(voices)
			}
			for _, v := range voices {
				fmt.Printf("%-46s %-4s %s\n", v["id"], v["gender"], v["category"])
			}
			return nil
		},
	})
	return cmd
}
```

（`server.BuiltinVoices()` 返回 `[]map[string]string`——在 Task 9 的 voices.go 里把 `builtinVoices` 包一层导出函数。）

`cmd/toolbox/tts.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/service"
	"github.com/yann0917/toolbox/internal/store"
)

func newTTSCommand() *cobra.Command {
	var (
		textFile   string
		voice      string
		format     string
		speedRatio float64
		volumeRatio float64
		outPath    string
		jsonOut    bool
	)
	cmd := &cobra.Command{
		Use:   "tts <text>",
		Short: "语音合成：文本转语音",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			text := ""
			if len(args) == 1 {
				text = args[0]
			}
			if textFile != "" {
				raw, err := os.ReadFile(textFile)
				if err != nil {
					return fmt.Errorf("读取文本文件失败: %w", err)
				}
				text = string(raw)
			}
			params := map[string]any{"text": text, "voice": voice, "format": format,
				"speed_ratio": speedRatio, "volume_ratio": volumeRatio}
			return runToolSync(c, "volcengine", "tts", params, outPath, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&textFile, "file", "", "从文件读取文本")
	f.StringVar(&voice, "voice", "zh_female_cancan_mars_bigtts", "音色 ID")
	f.StringVar(&format, "format", "mp3", "音频格式: mp3|wav|pcm|ogg_opus")
	f.Float64Var(&speedRatio, "speed-ratio", 1.0, "语速 0.2-3.0")
	f.Float64Var(&volumeRatio, "volume-ratio", 1.0, "音量 0.2-3.0")
	f.StringVar(&outPath, "out", "", "产物输出路径（默认数据目录自动命名）")
	f.BoolVar(&jsonOut, "json", false, "stdout 输出机器可读 JSON")
	return cmd
}

// runToolSync 同步执行工具：CLI 共享入口，处理 --out 重定位与 JSON/退出码。
func runToolSync(c *cobra.Command, providerName, toolName string, params map[string]any, outPath string, jsonOut bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := service.New(cfg)
	if err != nil {
		return err
	}
	defer svc.Close()
	svc.StartEngine(nil, 1)

	if outPath != "" {
		params["_out"] = outPath // provider 侧支持 _out 参数指定产物绝对路径
	}
	task, arts, err := svc.Engine().SubmitSync(c.Context(), providerName, toolName, params, nil)
	if err != nil {
		eprintf("错误: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
	result := jsonResult{
		TaskID: task.ID, Provider: task.Provider, Tool: task.Tool,
		Status: string(task.Status), CostMS: task.CostMS,
	}
	for _, a := range arts {
		result.Artifacts = append(result.Artifacts, artifactOut{
			Kind: a.Kind, Path: a.Path, Format: a.Format, Size: a.Size, DurationMS: a.DurationMS,
		})
	}
	// summary 从产物/任务参数恢复（Task 7 中 Run 已返回；此处从 artifact meta 汇总）
	if jsonOut {
		printJSON(result)
	} else {
		eprintf("完成，耗时 %dms\n", task.CostMS)
		for _, a := range result.Artifacts {
			fmt.Printf("%s: %s\n", a.Kind, a.Path)
		}
	}
	_ = store.StatusSucceeded
	return nil
}
```

说明：`_out` 参数让产物写到指定路径——`TTSTool.Run` 需支持：`if p, ok := in.Params["_out"].(string); ok && p != "" { absPath = p; relPath = p }`（Task 7 实现时加上这个分支；`_out` 不进 ParamSpecs）。同时 `jsonResult.Summary` 在 `SubmitSync` 返回的 `TaskOutput.Summary` 里——但 `Engine.run` 丢弃了它。**修正 Task 5**：`run()` 把 `out.Summary` JSON 序列化存入 `task.Params` 之外的字段？最小改动：给 `store.Task` 增加 `Summary string`（`gorm:"type:text"`），`run()` 里 `t.Summary, _ = json.Marshal(out.Summary)`，CLI 从 `task.Summary` 恢复。Task 3 的 AutoMigrate 与模型同步补该字段。

root.go 注册子命令：

```go
func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "toolbox", Short: "多媒体 AI 工具箱", Version: version}
	root.AddCommand(
		newConfigCmd(),
		newVoicesCmd(),
		newTTSCommand(),
		newServeCommand(), // Task 14 实现，先返回占位错误 "serve 将在 Web 集成后可用"
	)
	return root
}
```

- [ ] **Step 4: 测试通过 + 手工冒烟**

Run: `go test ./cmd/toolbox/ -v && go run ./cmd/toolbox --help`
Expected: PASS；帮助输出包含 tts/config/voices/serve

- [ ] **Step 5: Commit**

```bash
git add cmd/toolbox
git commit -m "feat: CLI config/voices/tts 与 --json 契约"
```

---

### Task 11: 前端骨架（Vite + React 19 + 主题 + 布局）

**Files:**
- Create: `web/`（Vite 脚手架）、`web/src/main.tsx`、`web/src/App.tsx`、`web/src/theme.css`、`web/src/lib/api.ts`、`web/src/lib/ws.ts`、`web/src/components/Layout.tsx`
- Create: `web/src/pages/WorkbenchPage.tsx`（空壳占位，Task 12 填充）

**Interfaces:**
- Consumes: REST `/api/*`（Task 9）。
- Produces: `api.ts` 导出 `fetchJSON<T>(path, init?)`；`ws.ts` 导出 `useTaskEvents()`（返回 TaskEvent 流的订阅 hook）；主题 CSS variables `--bg/--surface/--border/--fg/--muted/--accent`，`.light` 类切换亮色。

- [ ] **Step 1: 脚手架与依赖**

```bash
cd /Users/yabo/wwwroot/toolbox
npm create vite@latest web -- --template react-ts
cd web && npm install
npm install react-router-dom @tanstack/react-query zustand
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 2: 主题与全局样式**

`web/src/theme.css`:

```css
@import "tailwindcss";

:root {
  --bg: #0a0a0b;
  --surface: #131316;
  --surface-hover: #1a1a1f;
  --border: rgba(255, 255, 255, 0.08);
  --fg: #ededf0;
  --muted: #8a8a93;
  --accent: #22d3ee;
  --accent-fg: #06181c;
  --danger: #f87171;
  --ok: #34d399;
  --warn: #fbbf24;
}
:root.light {
  --bg: #f7f7f8;
  --surface: #ffffff;
  --surface-hover: #f0f0f2;
  --border: rgba(0, 0, 0, 0.09);
  --fg: #17171a;
  --muted: #6b6b74;
  --accent: #0891b2;
  --accent-fg: #ffffff;
}
body {
  background: var(--bg);
  color: var(--fg);
  font-family: Inter, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  font-variant-numeric: tabular-nums;
}
```

`web/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import Layout from "./components/Layout";
import WorkbenchPage from "./pages/WorkbenchPage";
import "./theme.css";

const qc = new QueryClient();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<WorkbenchPage />} />
            <Route path="/tts" element={<WorkbenchPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
```

- [ ] **Step 3: API 与 WS 封装**

`web/src/lib/api.ts`:

```ts
const base = import.meta.env.DEV ? "http://localhost:8080" : "";

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(base + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error((body as { error?: string }).error ?? `HTTP ${resp.status}`);
  }
  return resp.json();
}

export interface ToolMeta { provider: string; name: string; title: string; description: string; group: string }
export interface ParamSpec { key: string; label: string; type: string; required: boolean; default?: unknown; options?: { value: string; label: string }[]; placeholder?: string; group?: string }
export interface ToolInfo { meta: ToolMeta; param_specs: ParamSpec[] }
export const listTools = () => fetchJSON<ToolInfo[]>("/api/tools");
```

`web/src/lib/ws.ts`:

```ts
import { useEffect, useState } from "react";

export interface TaskEvent {
  type: "progress" | "done" | "error" | "canceled" | "task.snapshot";
  task_id?: string;
  progress?: number;
  note?: string;
  error?: string;
  tasks?: unknown[];
}

export function useTaskEvents(): TaskEvent | null {
  const [last, setLast] = useState<TaskEvent | null>(null);
  useEffect(() => {
    const url = (import.meta.env.DEV ? "ws://localhost:8080" : `ws://${location.host}`) + "/api/ws";
    let retry: ReturnType<typeof setTimeout>;
    let ws: WebSocket;
    const connect = () => {
      ws = new WebSocket(url);
      ws.onmessage = (e) => setLast(JSON.parse(e.data));
      ws.onclose = () => { retry = setTimeout(connect, 2000); };
    };
    connect();
    return () => { clearTimeout(retry); ws.close(); };
  }, []);
  return last;
}
```

- [ ] **Step 4: 布局与主题切换**

`web/src/components/Layout.tsx`:

```tsx
import { useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";

const nav = [
  { to: "/", label: "工作台", icon: "◎" },
  { to: "/tts", label: "语音合成", icon: "🗣" },
  { to: "/history", label: "历史", icon: "🕘" },
  { to: "/settings", label: "设置", icon: "⚙" },
];

export default function Layout() {
  const [theme, setTheme] = useState<"dark" | "light">(
    () => (localStorage.getItem("theme") as "dark" | "light") ?? "dark"
  );
  useEffect(() => {
    document.documentElement.classList.toggle("light", theme === "light");
    localStorage.setItem("theme", theme);
  }, [theme]);

  return (
    <div className="flex h-screen">
      <aside className="w-56 border-r border-[var(--border)] bg-[var(--surface)] p-4 flex flex-col gap-1">
        <div className="text-lg font-semibold px-2 pb-4">toolbox</div>
        {nav.map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            className={({ isActive }) =>
              `px-3 py-2 rounded-lg text-sm transition-colors ${
                isActive ? "bg-[var(--surface-hover)] text-[var(--accent)]" : "text-[var(--muted)] hover:text-[var(--fg)]"
              }`
            }
          >
            <span className="mr-2">{n.icon}</span>{n.label}
          </NavLink>
        ))}
        <div className="mt-auto">
          <button
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            className="w-full px-3 py-2 rounded-lg text-sm text-[var(--muted)] hover:text-[var(--fg)] border border-[var(--border)]"
          >
            {theme === "dark" ? "☀ 亮色" : "☾ 暗色"}
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto p-8">
        <Outlet />
      </main>
    </div>
  );
}
```

`web/src/App.tsx`:

```tsx
export default function App() {
  return null; // 路由在 main.tsx；保留文件满足脚手架引用
}
```

（Vite 模板自带的 `App.css`/`index.css`/logo 删除；`vite.config.ts` 加 Tailwind 插件与 dev 代理：）

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy: { "/api": "http://localhost:8080" } },
});
```

（若 dev 代理生效，`api.ts`/`ws.ts` 的 `base` 可简化为空串——保留显式 base 以便后端不在 8080 时调试。）

- [ ] **Step 5: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功无类型错误

- [ ] **Step 6: Commit**

```bash
git add web
git commit -m "feat: 前端骨架（React19+Tailwind v4 双主题+布局）"
```

---

### Task 12: 前端 TTS 工作台与历史

**Files:**
- Create: `web/src/pages/TTSPage.tsx`, `web/src/pages/HistoryPage.tsx`, `web/src/components/MiniPlayer.tsx`, `web/src/components/TaskProgress.tsx`
- Modify: `web/src/main.tsx`（路由挂上 `/tts`、`/history`）

**Interfaces:**
- Consumes: Task 9 REST、Task 11 `useTaskEvents/fetchJSON`。
- Produces: `MiniPlayer({src, title})` 音频播放组件；`TaskProgress({task})` 进度卡片。

- [ ] **Step 1: TTS 页（表单 + 提交 + 进度）**

`web/src/pages/TTSPage.tsx`:

```tsx
import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { fetchJSON, type ToolInfo } from "../lib/api";
import TaskProgress from "../components/TaskProgress";
import MiniPlayer from "../components/MiniPlayer";
import { useTaskEvents } from "../lib/ws";

interface Task { id: string; status: string; progress: number; progress_note: string; error?: string }

export default function TTSPage() {
  const [text, setText] = useState("");
  const [voice, setVoice] = useState("zh_female_cancan_mars_bigtts");
  const [format, setFormat] = useState("mp3");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<Task | null>(null);
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const qc = useQueryClient();
  const ev = useTaskEvents();

  // WS 事件驱动当前任务进度与完成
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") setTask((t) => ({ ...(t ?? { id: taskId, status: "running" } as Task), progress: ev.progress ?? 0, progress_note: ev.note ?? "" }));
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: Task; artifacts: { id: string; kind: string; filename: string }[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          const audio = d.artifacts.find((a) => a.kind === "audio");
          if (audio) setAudioUrl(`/api/artifacts/${audio.id}/stream`);
        });
    }
  }, [ev, taskId]);

  const submit = useMutation({
    mutationFn: () =>
      fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({ provider: "volcengine", tool: "tts", params: { text, voice, format, speed_ratio: 1, volume_ratio: 1 } }),
      }),
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setTask({ id: d.task_id, status: "pending", progress: 0, progress_note: "已提交" });
      setAudioUrl(null);
      qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => alert(e.message),
  });

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">语音合成</h1>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="输入要合成的文本…"
          rows={8}
          className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]"
        />
        <div className="flex items-center gap-3 text-sm">
          <label className="text-[var(--muted)]">音色</label>
          <input value={voice} onChange={(e) => setVoice(e.target.value)}
            className="flex-1 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2" />
          <label className="text-[var(--muted)]">格式</label>
          <select value={format} onChange={(e) => setFormat(e.target.value)}
            className="rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {["mp3", "wav", "pcm", "ogg_opus"].map((f) => <option key={f}>{f}</option>)}
          </select>
        </div>
        <button
          disabled={!text.trim() || submit.isPending}
          onClick={() => submit.mutate()}
          className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40"
        >
          {submit.isPending ? "提交中…" : "开始合成"}
        </button>
      </div>
      {task && <TaskProgress task={task} />}
      {audioUrl && <MiniPlayer src={audioUrl} title="合成结果" />}
    </div>
  );
}
```

`web/src/components/TaskProgress.tsx`:

```tsx
export default function TaskProgress({ task }: { task: { status: string; progress: number; progress_note: string; error?: string } }) {
  const color = task.status === "succeeded" ? "var(--ok)" : task.status === "failed" ? "var(--danger)" : "var(--accent)";
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 space-y-2">
      <div className="flex justify-between text-sm">
        <span>{task.progress_note || "处理中"}</span>
        <span style={{ color }}>{task.status}</span>
      </div>
      <div className="h-1.5 rounded-full bg-[var(--surface-hover)] overflow-hidden">
        <div className="h-full rounded-full transition-all"
          style={{ width: `${task.progress}%`, background: color }} />
      </div>
      {task.error && <p className="text-xs" style={{ color: "var(--danger)" }}>{task.error}</p>}
    </div>
  );
}
```

`web/src/components/MiniPlayer.tsx`:

```tsx
export default function MiniPlayer({ src, title }: { src: string; title: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 flex items-center gap-3">
      <audio controls src={src} className="flex-1" />
      <span className="text-sm text-[var(--muted)] shrink-0">{title}</span>
    </div>
  );
}
```

- [ ] **Step 2: 历史页（列表 + 播放 + 下载 + 删除）**

`web/src/pages/HistoryPage.tsx`:

```tsx
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { fetchJSON } from "../lib/api";
import MiniPlayer from "../components/MiniPlayer";

interface Task { id: string; provider: string; tool: string; status: string; cost_ms: number; created_at: string }
interface Artifact { id: string; kind: string; filename: string }
interface TaskDetail { task: Task; artifacts: Artifact[] }

export default function HistoryPage() {
  const qc = useQueryClient();
  const [playing, setPlaying] = useState<string | null>(null);
  const { data } = useQuery({
    queryKey: ["tasks"],
    queryFn: () => fetchJSON<{ items: Task[]; total: number }>("/api/tasks"),
  });
  const del = useMutation({
    mutationFn: (id: string) => fetchJSON(`/api/tasks/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
  });
  const detail = useQuery({
    queryKey: ["task", playing],
    enabled: !!playing,
    queryFn: () => fetchJSON<TaskDetail>(`/api/tasks/${playing}`),
  });

  return (
    <div className="max-w-4xl mx-auto space-y-4">
      <h1 className="text-xl font-semibold">历史任务</h1>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] divide-y divide-[var(--border)]">
        {(data?.items ?? []).map((t) => (
          <div key={t.id} className="p-4 flex items-center gap-3 text-sm">
            <span className="w-20 text-[var(--muted)]">{t.tool}</span>
            <span className="w-24">{t.status}</span>
            <span className="flex-1 text-[var(--muted)]">{t.created_at}</span>
            <button onClick={() => setPlaying(t.id)} className="text-[var(--accent)]">查看</button>
            <a href={`${"http://localhost:8080"}/api/tasks/${t.id}`} onClick={(e) => e.preventDefault()} className="hidden" />
            <button onClick={() => del.mutate(t.id)} className="text-[var(--danger)]">删除</button>
          </div>
        ))}
      </div>
      {detail.data && (
        <div className="space-y-2">
          {detail.data.artifacts.map((a) => (
            <div key={a.id} className="flex items-center gap-3">
              {a.kind === "audio" && <MiniPlayer src={`/api/artifacts/${a.id}/stream`} title={a.filename} />}
              <a href={`/api/artifacts/${a.id}/download`} className="text-sm text-[var(--accent)]">下载 {a.filename}</a>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

main.tsx 路由更新：

```tsx
import HistoryPage from "./pages/HistoryPage";
// Routes 内加：
<Route path="/history" element={<HistoryPage />} />
```

（TTS 路由已在 Task 11 挂 `/tts`；`/` 与 `/tts` 均渲染 WorkbenchPage 的问题：把 `/` 改为新的 WorkbenchPage 空壳，`/tts` 改为 TTSPage。）

- [ ] **Step 3: 构建验证**

Run: `cd web && npm run build`
Expected: 成功

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat: 前端 TTS 工作台与历史页"
```

---

### Task 13: 前端设置页

**Files:**
- Create: `web/src/pages/SettingsPage.tsx`
- Modify: `web/src/main.tsx`（挂 `/settings`）

**Interfaces:**
- Consumes: `GET/PUT /api/settings`、`POST /api/settings/test-connection`（Task 9）。

- [ ] **Step 1: 实现设置页**

```tsx
import { useQuery, useMutation } from "@tanstack/react-query";
import { fetchJSON } from "../lib/api";

interface Settings { volc: { speech: { app_id: string; has_access_token: boolean; api_key: string }; mediakit: { has_api_key: boolean } }; data_dir: string }

export default function SettingsPage() {
  const { data, refetch } = useQuery({ queryKey: ["settings"], queryFn: () => fetchJSON<Settings>("/api/settings") });
  const save = useMutation({
    mutationFn: (body: Record<string, string>) =>
      fetchJSON("/api/settings", { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => refetch(),
  });
  const test = useMutation({
    mutationFn: () => fetchJSON<{ ok: boolean; message: string }>("/api/settings/test-connection", { method: "POST" }),
  });

  const field = (name: string, label: string, placeholder: string) => (
    <label className="block space-y-1">
      <span className="text-sm text-[var(--muted)]">{label}</span>
      <input name={name} placeholder={placeholder} defaultValue=""
        className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2 text-sm" />
    </label>
  );

  const onSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    save.mutate({
      app_id: String(fd.get("app_id") ?? ""),
      access_token: String(fd.get("access_token") ?? ""),
      api_key: String(fd.get("api_key") ?? ""),
      mediakit_api_key: String(fd.get("mediakit_api_key") ?? ""),
    });
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">设置</h1>
      <form onSubmit={onSubmit} className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <div className="text-sm">
          APP ID：<span className="text-[var(--muted)]">{data?.volc.speech.app_id || "未配置"}</span>
          ，Access Token：{data?.volc.speech.has_access_token ? "已配置" : <span className="text-[var(--warn)]">未配置</span>}
        </div>
        {field("app_id", "火山语音 APP ID", "留空则不修改")}
        {field("access_token", "Access Token", "留空则不修改")}
        {field("api_key", "新版 API Key（二选一）", "留空则不修改")}
        <button className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium">保存</button>
        {save.isSuccess && <p className="text-xs" style={{ color: "var(--ok)" }}>已保存，重启服务后生效</p>}
      </form>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-3">
        <button type="button" onClick={() => test.mutate()}
          className="px-5 py-2 rounded-lg border border-[var(--border)] text-sm">测试语音凭证连通性</button>
        {test.data && <p className="text-sm" style={{ color: test.data.ok ? "var(--ok)" : "var(--danger)" }}>{test.data.message}</p>}
        <p className="text-xs text-[var(--muted)]">数据目录：{data?.data_dir}</p>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 构建验证**

Run: `cd web && npm run build`
Expected: 成功

- [ ] **Step 3: Commit**

```bash
git add web/src
git commit -m "feat: 设置页（凭证配置与连通性测试）"
```

---

### Task 14: embed 集成 + serve 命令 + 端到端冒烟

**Files:**
- Create: `cmd/toolbox/serve.go`
- Modify: `internal/server/server.go`（静态资源）
- Modify: `Makefile`

**Interfaces:**
- Consumes: service/server（Task 8/9）、前端产物 `web/dist`。
- Produces: `toolbox serve [--port 8080]`：启动后 `http://localhost:8080` 提供前端页面与 API。

- [ ] **Step 1: embed 静态资源**

`cmd/toolbox/serve.go`:

```go
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/server"
	"github.com/yann0917/toolbox/internal/service"
)

//go:embed all:webdist
var webDist embed.FS

func newServeCommand() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 Web 控制台",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if port != 0 {
				cfg.Server.Port = port
			}
			svc, err := service.New(cfg)
			if err != nil {
				return err
			}
			defer svc.Close()
			srv := server.New(svc)
			svc.StartEngine(srv.Hub().Notify, 2)

			handler := server.WithStatic(srv.Handler(), webDist)
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.Server.Port)
			fmt.Fprintf(os.Stderr, "toolbox Web 已启动: http://%s\n", addr)
			return http.ListenAndServe(addr, handler)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "端口（默认取配置）")
	return cmd
}
```

embed 路径问题：`//go:embed` 不能引用包目录外的 `web/dist`。标准做法：在 `cmd/toolbox/` 下建 `webdist` 目录并在构建时复制产物，或把 embed 放在 `internal/server/web.go` + `web` 目录下。**定案**：Makefile 构建时 `rsync web/dist/ cmd/toolbox/webdist/`，`webdist` 内容不进 git（.gitignore 追加 `cmd/toolbox/webdist/`，但需要 `.gitkeep` 或构建兜底——`go:embed all:webdist` 在目录不存在时编译失败，所以仓库内提交一个 `cmd/toolbox/webdist/index.html` 占位文件，内容为「请先执行 make web」提示页）。

`internal/server/static.go`:

```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// WithStatic 在 API handler 外挂 SPA 静态资源（embed 产物），非 /api 路径回退 index.html。
func WithStatic(api http.Handler, dist embed.FS) http.Handler {
	sub, err := fs.Sub(dist, "webdist")
	if err != nil {
		return api
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			// SPA 入口直接交给文件服务器
		}
		fileServer.ServeHTTP(w, r)
	})
}

var _ = gin.Default // 保持 gin import（若未用则删除此行与 import）
```

（实现时若 `gin` 未被此文件使用则删除该 import 与占位行。SPA 路由 fallback：`fileServer` 找不到文件时返回 404——前端用 BrowserRouter 需要 fallback 到 index.html；实现方式：包一层 `http.HandlerFunc`，先 `fs.Stat` 判断存在性，不存在则 `r.URL.Path = "/"`。）

- [ ] **Step 2: Makefile 更新**

```makefile
.PHONY: build test web all clean

web:
	cd web && npm ci && npm run build
	rm -rf cmd/toolbox/webdist && mkdir -p cmd/toolbox/webdist
	cp -r web/dist/* cmd/toolbox/webdist/

build:
	go build -o bin/toolbox ./cmd/toolbox

test:
	go test ./...

all: web build

clean:
	rm -rf bin cmd/toolbox/webdist
```

- [ ] **Step 3: 端到端冒烟（手动，需要真实凭证；无凭证时验证到错误提示为止）**

```bash
make all
./bin/toolbox config set volc.speech.app_id "<真实APP ID>"   # 用户输入
./bin/toolbox config set volc.speech.access_token "<真实Token>"
./bin/toolbox tts "你好，toolbox" --out /tmp/tts-smoke.mp3 --json
# Expected: stdout 单行 JSON，artifacts[0].path = /tmp/tts-smoke.mp3；文件存在可播放

./bin/toolbox serve --port 8080 &
# 浏览器打开 http://127.0.0.1:8080 → TTS 页合成 → 播放 → 历史可见
# 无凭证场景：./bin/toolbox tts "hi" → 退出码 4，stderr 提示 config set
```

- [ ] **Step 4: 更新 README 开发状态与提交**

```bash
# README.md「开发状态」改为：M1+M2 已完成（骨架 + TTS 端到端）；ASR/播客/人声分离待实施
git add -A
git commit -m "feat: serve 命令与前端 embed，TTS 端到端打通"
```

---

## 后续里程碑（另行出计划）

- **M3 ASR**：流式 WS 客户端（本地文件分片）、submit/query 异步通道、`POST /api/uploads`、识别结果页（分句/时间戳/跳播）、TXT/SRT 导出。
- **M4 播客**：播客 WS V3 客户端（事件 360-363、断点续传、audio_url 转存）、播客工坊三步向导、对话流进度视图。
- **M5 人声分离**：MediaKit REST 客户端、分离页、产物联动（分离→ASR）。
- **M6 产品化**：自研波形播放器、全局 mini-player、工作台统计、错误文案打磨、goreleaser 发布。

## Self-Review 记录

1. **Spec 覆盖**：M1（config/store/provider/task/server 骨架）→ Task 1-5、9；M2（TTS 端到端）→ Task 6-14；`--json`/退出码契约 → Task 10；voices → Task 9/10；设置+连通性 → Task 9/13；embed/serve → Task 14。ASR/播客/分离/播放器打磨明确划入后续里程碑（设计文档 M3-M6）。
2. **占位符扫描**：无 TBD/TODO；音色表 `builtinVoices` 的 10 个真实 ID 需实现时从火山官方音色列表文档取值（已在 Task 9 注明取值来源，非占位——数据填充动作明确）。
3. **类型一致性**：`store.Task.Summary`（Task 5 修正引入）需同步进 Task 3 模型——Task 3 实现时按 Task 5 的修正说明加上 `Summary string` 字段；`provider.ProgressReporter`、`task.Event`、`jsonResult` 的字段名在 Task 5/9/10 间已互相对齐；`TTSTool` 构造签名 `(cred, outDir)` 在 Task 7/8 一致。
