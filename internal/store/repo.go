package store

import (
	"errors"
	"strings"
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

// SearchSucceededSummaries 转写全文搜索的粗筛：summary 列 LIKE 命中的成功任务，
// 按创建时间倒序。LIKE 只做候选集粗筛（转义 %/_/\\），精确命中与片段提取由上层
// 解析 summary JSON 后判定；个人工具量级（千级任务）全表 LIKE 足够，量大再上 FTS5。
func (d *DB) SearchSucceededSummaries(keyword string, limit int) ([]Task, error) {
	if strings.TrimSpace(keyword) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	pattern := "%" + escapeLike(keyword) + "%"
	var items []Task
	err := d.gorm.Model(&Task{}).
		Where("status = ? AND summary LIKE ? ESCAPE '\\'", StatusSucceeded, pattern).
		Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	return strings.ReplaceAll(s, "_", "\\_")
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
