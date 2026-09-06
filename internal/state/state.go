package state

import (
	"errors"
	"fmt"
	"sync"
)

const (
	MaxKeyLen   = 4096
	MaxValueLen = 1 << 20 // 1 MiB

	DefaultTotalLimit = 64 << 20 // 64 MiB
)

type Budget struct {
	mu    sync.RWMutex
	limit int64
	used  int64
}

func NewBudget(limit int64) *Budget {
	return &Budget{limit: limit}
}

// bounded string-to-string map
type Store struct {
	budget *Budget
	data   map[string]string
}

func NewStore(budget *Budget) *Store {
	if budget == nil {
		budget = NewBudget(DefaultTotalLimit)
	}
	return &Store{budget: budget}
}

func (s *Store) Budget() *Budget {
	return s.budget
}

func (s *Store) Get(key string) (string, bool) {
	s.budget.mu.RLock()
	defer s.budget.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *Store) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}

func (s *Store) Set(key, value string) error {
	switch {
	case key == "":
		return errors.New("key must not be empty")
	case len(key) > MaxKeyLen:
		return fmt.Errorf("key exceeds maximum size of %d bytes", MaxKeyLen)
	case len(value) > MaxValueLen:
		return fmt.Errorf("value exceeds maximum size of %d bytes", MaxValueLen)
	}

	s.budget.mu.Lock()
	defer s.budget.mu.Unlock()

	delta := int64(len(value) - len(s.data[key]))
	if s.budget.used+delta > s.budget.limit {
		return fmt.Errorf("state limit of %d bytes exceeded", s.budget.limit)
	}
	s.budget.used += delta
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = value
	return nil
}

func (s *Store) Delete(key string) {
	s.budget.mu.Lock()
	defer s.budget.mu.Unlock()
	if old, ok := s.data[key]; ok {
		s.budget.used -= int64(len(old))
		delete(s.data, key)
	}
}
