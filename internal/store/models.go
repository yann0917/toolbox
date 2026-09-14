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
	Input        string     `gorm:"type:text"` // 原始输入引用 JSON（file_ids/artifact_input），供重跑与回放溯源；CLI 直传本地路径时为空
	Summary      string     `gorm:"type:text"` // 任务完成摘要 JSON（provider.TaskOutput.Summary 序列化）
	Progress     int
	ProgressNote string
	Error        string `gorm:"type:text"`
	CostMS       int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Artifact struct {
	ID         string `gorm:"primaryKey;size:36"`
	TaskID     string `gorm:"size:36;index"`
	Kind       string `gorm:"size:16"` // audio|transcript|dialog|subtitle|translation|minutes
	Path       string // data 目录相对路径；_out 重定向时可为绝对路径
	Filename   string `gorm:"size:255"`
	Format     string `gorm:"size:16"`
	Size       int64
	DurationMS int64
	Meta       string `gorm:"type:text"` // JSON
	CreatedAt  time.Time
}
