package idempotency

import (
	"container/list"
	"context"
	"sync"
	"time"
)
type MemoryEntry struct {
	key            string
	fingerprint    string
	response       *Response
	waiters        []chan *Response
	expirationTime time.Time
	listElements   *list.Element
}
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]*MemoryEntry
	lruList *list.List
	maxSize int
}
func NewMemoryStore(maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &MemoryStore{
		entries: make(map[string]*MemoryEntry),
		lruList: list.New(),
		maxSize: maxSize,
	}
}
func (s *MemoryStore) Start(ctx context.Context, key string, fingerprint string, ttl time.Duration) (StartStatus, *Response, <-chan *Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evict()
	value, exists := s.entries[key]
	if !exists {
		newEntry := &MemoryEntry{
			key:            key,
			fingerprint:    fingerprint,
			waiters:        make([]chan *Response, 0),
			expirationTime: time.Now().Add(ttl),
		}
		s.entries[key] = newEntry
		newEntry.listElements = s.lruList.PushFront(newEntry)
		s.evict()
		return StatusMiss, nil, nil, nil
	}
	if value.fingerprint != fingerprint {
		return 0, nil, nil, ErrFingerPrintMismatch
	}
	if time.Now().After(value.expirationTime) {
		value.fingerprint = fingerprint
		value.response = nil
		value.waiters = make([]chan *Response, 0)
		value.expirationTime = time.Now().Add(ttl)
		s.lruList.MoveToFront(value.listElements)
		return StatusMiss, nil, nil, nil
	}
	if value.response == nil {
		ch := make(chan *Response, 1)
		value.waiters = append(value.waiters, ch)
		return StatusInProgress, nil, ch, nil
	}
	s.lruList.MoveToFront(value.listElements)
	return StatusHit, value.response, nil, nil
}

func (s *MemoryStore) Complete(ctx context.Context, key string, fingerprint string, resp *Response, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.entries[key]
	if !exists || value.fingerprint != fingerprint {
		return nil
	}
	if resp == nil {
		for _, ch := range value.waiters {
			select {
			case ch <- nil:
			default:
			}
		}
		delete(s.entries, key)
		s.lruList.Remove(value.listElements)
		return nil
	}
	value.response = resp
	value.expirationTime = time.Now().Add(ttl)
	s.lruList.MoveToFront(value.listElements)
	for _, ch := range value.waiters {
		select {
		case ch <- resp:
		default:
		}
	}
	value.waiters = nil
	return nil
}

func (s *MemoryStore) evict() {
	now := time.Now()
	for s.lruList.Len() > 0 {
		back := s.lruList.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*MemoryEntry)
		if now.After(entry.expirationTime) {
			delete(s.entries, entry.key)
			s.lruList.Remove(back)
		} else {
			break
		}
	}
	for s.lruList.Len() > s.maxSize {
		back := s.lruList.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*MemoryEntry)
		delete(s.entries, entry.key)
		s.lruList.Remove(back)
	}
}