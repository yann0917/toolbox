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
	ID         string `gorm:"primaryKey;size:36"`
	TaskID     string `gorm:"size:36;index"`
	Kind       string `gorm:"size:16"` // audio|transcript|dialog|subtitle
	Path       string // data 目录相对路径
	Filename   string `gorm:"size:255"`
	Format     string `gorm:"size:16"`
	Size       int64
	DurationMS int64
	Meta       string `gorm:"type:text"` // JSON
	CreatedAt  time.Time
}
