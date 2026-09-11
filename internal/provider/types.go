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
