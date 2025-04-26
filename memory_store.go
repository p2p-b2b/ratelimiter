package ratelimiter

import "sync"

type InMemoryStorage struct {
	data sync.Map
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: sync.Map{},
	}
}

func (s *InMemoryStorage) Store(key any, value any) {
	s.data.Store(key, value)
}

func (s *InMemoryStorage) Delete(key any) {
	s.data.Delete(key)
}

func (s *InMemoryStorage) Load(key any) (value any, ok bool) {
	value, ok = s.data.Load(key)
	return
}
