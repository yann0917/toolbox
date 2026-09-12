package server

import (
	"encoding/json"

	"github.com/yann0917/toolbox/internal/provider"
	"github.com/yann0917/toolbox/internal/store"
)

type taskDTO struct {
	ID        string          `json:"id"`
	Provider  string          `json:"provider"`
	Tool      string          `json:"tool"`
	Status    string          `json:"status"`
	Progress  int             `json:"progress"`
	Note      string          `json:"progress_note"`
	Error     string          `json:"error,omitempty"`
	CostMS    int64           `json:"cost_ms"`
	Params    json.RawMessage `json:"params"`
	Summary   json.RawMessage `json:"summary,omitempty"` // 任务完成摘要 JSON（ASR segments 等）
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
		CostMS: t.CostMS, Params: json.RawMessage(t.Params), Summary: json.RawMessage(t.Summary),
		CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toArtifactDTO(a store.Artifact) artifactDTO {
	return artifactDTO{
		ID: a.ID, Kind: a.Kind, Filename: a.Filename, Format: a.Format,
		Size: a.Size, DurationMS: a.DurationMS, Meta: json.RawMessage(a.Meta),
	}
}

type toolDTO struct {
	Meta       provider.ToolMeta    `json:"meta"`
	ParamSpecs []provider.ParamSpec `json:"param_specs"`
}
