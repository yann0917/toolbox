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
