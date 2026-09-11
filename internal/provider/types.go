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
	Value string `json:"value"`
	Label string `json:"label"`
}

type ParamSpec struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        ParamType     `json:"type"`
	Required    bool          `json:"required"`
	Default     any           `json:"default,omitempty"`
	Options     []ParamOption `json:"options,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Group       string        `json:"group,omitempty"`
}

type ToolMeta struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

type TaskInput struct {
	Params map[string]any
	Files  map[string]string
}

type Artifact struct {
	Kind       string         `json:"kind"` // audio|transcript|dialog|subtitle
	Path       string         `json:"path"` // data 目录相对路径；_out 重定向时可为绝对路径
	Format     string         `json:"format"`
	Size       int64          `json:"size"`
	DurationMS int64          `json:"duration_ms"`
	Meta       map[string]any `json:"meta,omitempty"`
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
