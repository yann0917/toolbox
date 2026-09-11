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
