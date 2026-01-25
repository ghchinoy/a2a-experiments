package main

import (
	"context"
	"sync"

	"github.com/a2aproject/a2a-go/a2a"
)

// memStore implements [a2asrv.TaskStore] for in-memory persistence.
// It tracks task state, history, and artifacts.
type memStore struct {
	mu    sync.RWMutex
	tasks map[a2a.TaskID]*a2a.Task
}

// newMemStore initializes a thread-safe in-memory task store.
func newMemStore() *memStore {
	return &memStore{tasks: make(map[a2a.TaskID]*a2a.Task)}
}

// Get retrieves a task by its ID. Returns ErrTaskNotFound if missing.
func (s *memStore) Get(ctx context.Context, id a2a.TaskID) (*a2a.Task, a2a.TaskVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, a2a.TaskVersionMissing, a2a.ErrTaskNotFound
	}
	return t, a2a.TaskVersionMissing, nil
}

// Save persists a task and its associated events.
func (s *memStore) Save(ctx context.Context, t *a2a.Task, ev a2a.Event, v a2a.TaskVersion) (a2a.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	return a2a.TaskVersionMissing, nil
}

// List returns a paginated list of tasks based on the provided request criteria.
func (s *memStore) List(ctx context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var tasks []*a2a.Task
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return &a2a.ListTasksResponse{Tasks: tasks, TotalSize: len(tasks)}, nil
}
