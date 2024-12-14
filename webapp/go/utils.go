package main

import (
	"sync"
	"time"
)

type KeyLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewKeyLocker initializes a new KeyLocker
func NewKeyLocker() *KeyLocker {
	return &KeyLocker{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock acquires a lock for the specified key
func (kl *KeyLocker) Lock(key string) {
	kl.mu.Lock()
	if _, exists := kl.locks[key]; !exists {
		kl.locks[key] = &sync.Mutex{}
	}
	lock := kl.locks[key]
	kl.mu.Unlock()
	lock.Lock()
}

// Unlock releases the lock for the specified key
func (kl *KeyLocker) Unlock(key string) {
	kl.mu.Lock()
	if lock, exists := kl.locks[key]; exists {
		lock.Unlock()
	}
	kl.mu.Unlock()
}

func longPolling(condition func() (bool, error), interval time.Duration, timeout time.Duration) (bool, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			ok, err := condition()
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		case <-timer.C:
			return false, nil
		}
	}
}
