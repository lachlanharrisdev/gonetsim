package state

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	s.budget.mu.RLock()
	defer s.budget.mu.RUnlock()
	_, ok := s.data[key]
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

// could move to a shared utils package but not necessary yet
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("size must be positive")
		}
		return n, nil
	}

	var mult int64
	switch {
	case strings.HasSuffix(s, "kib"):
		mult, s = 1<<10, s[:len(s)-3]
	case strings.HasSuffix(s, "mib"):
		mult, s = 1<<20, s[:len(s)-3]
	case strings.HasSuffix(s, "gib"):
		mult, s = 1<<30, s[:len(s)-3]
	case strings.HasSuffix(s, "k"):
		mult, s = 1<<10, s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		mult, s = 1<<20, s[:len(s)-1]
	case strings.HasSuffix(s, "g"):
		mult, s = 1<<30, s[:len(s)-1]
	default:
		return 0, fmt.Errorf("invalid size %q (expected e.g. 64MiB)", s)
	}

	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q (expected e.g. 64MiB)", s)
	}
	return n * mult, nil
}
