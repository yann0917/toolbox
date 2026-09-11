// Package server 提供 gin HTTP 服务与 WebSocket hub。
package server

import (
	"encoding/json"

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
