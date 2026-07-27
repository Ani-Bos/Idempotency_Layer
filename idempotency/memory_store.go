package idempotency

import (
	"container/list"
	"time"
	"sync"
	"context"
)

type MemoryEntry struct {
	key            string
	fingerprint    string
	response       *Response
	done chan *Response
	expirationTime time.Time
	listElements *list.Element
}

type MemoryStore struct {
	mu sync.Mutex
	entries map[string]*MemoryEntry
	lruList *list.List
	maxSize int
}

 func NewMemoryStore(maxSize int) *MemoryStore {
	if maxSize<=0{
		maxSize=10000
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
	s.evictExpiredEntries()
    value, exists := s.entries[key]
	if !exists {
       new_entry := &MemoryEntry{
		key: key,
		fingerprint: fingerprint,
		done: make(chan *Response, 1),
		expirationTime: time.Now().Add(ttl),
	   }
		s.entries[key] = new_entry
		s.lruList.PushFront(new_entry)
		new_entry.listElements = s.lruList.Front()
		return StatusMiss, nil, nil, nil
	}
	if value.fingerprint != fingerprint {
		return 0,nil,nil, ErrFingerPrintMismatch
	}
	if time.Now().After(value.expirationTime) {
		value.fingerprint = fingerprint
		value.response = nil
		value.done = make(chan *Response, 1)
		value.expirationTime = time.Now().Add(ttl)
		s.lruList.MoveToFront(value.listElements)
		return StatusMiss, nil, nil, nil
	}

	if value.response != nil {
		return StatusInProgress, nil,value.done, nil
	}

	s.lruList.MoveToFront(value.listElements)
	return StatusHit, value.response, nil, nil
}


func (s *MemoryStore) Complete(ctx context.Context, key string, fingerprint string, resp *Response, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.entries[key]
	if !exists {
		return nil
	}
	if value.fingerprint != fingerprint {
		return nil
	}
	if resp == nil {
		delete(s.entries,key)
		s.lruList.Remove(value.listElements)
		return nil
	}
	value.response = resp
	value.expirationTime = time.Now().Add(ttl)
	s.lruList.MoveToFront(value.listElements)
	select {
	case value.done <- resp:
	default:
	}
	return nil
}

func (s *MemoryStore) evictExpiredEntries() {
	time_now := time.Now()
	n:= s.lruList.Len()
	for i := 0; i < n; i++ {
		element := s.lruList.Back()
		if element == nil {
			break
		}
		entry := element.Value.(*MemoryEntry)
		if time_now.After(entry.expirationTime) {
			delete(s.entries, entry.key)
			s.lruList.Remove(element)
		} else {
			break
		}
	}
	for s.lruList.Len() > s.maxSize {
		element := s.lruList.Back()
		if element == nil {
			break
		}
		entry := element.Value.(*MemoryEntry)
		delete(s.entries, entry.key)
		s.lruList.Remove(element)
	}
}