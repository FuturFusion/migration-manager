package util

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/FuturFusion/migration-manager/internal/logger"
)

// RunConcurrentList runs the given function concurrently for each entity in the given list.
// Any encountered errors will be logged, and when the run finishes, the last encountered error is returned.
func RunConcurrentList[T any](entities []T, f func(T) error) error {
	if len(entities) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	errs := make([]error, 0, len(entities))

	for _, e := range entities {
		wg.Add(1)

		go func(e T) {
			defer wg.Done()
			err := f(e)
			if err != nil {
				slog.Error("Failed concurrent action", logger.Err(err))
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(e)
	}

	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("Failed to run %d concurrent actions. Last error: %w", len(errs), errs[len(errs)-1])
	}

	return nil
}

// RunConcurrentMap runs the given function concurrently for each entity in the given map.
// Any encountered errors will be logged, and when the run finishes, the last encountered error is returned.
func RunConcurrentMap[K comparable, V any](entities map[K]V, f func(K, V) error) error {
	if len(entities) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	errs := make([]error, 0, len(entities))
	for k, v := range entities {
		wg.Add(1)

		go func(k K, v V) {
			defer wg.Done()
			err := f(k, v)
			if err != nil {
				slog.Error("Failed concurrent action", logger.Err(err))
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(k, v)
	}

	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("Failed to run %d concurrent actions. Last error: %w", len(errs), errs[len(errs)-1])
	}

	return nil
}

type IDLock[T comparable] struct {
	accessLock sync.Mutex
	lockMap    map[T]*sync.Mutex
}

// NewIDLock creates a thread-safe map of sync.Mutexes keyed by ID.
func NewIDLock[T comparable]() IDLock[T] {
	return IDLock[T]{lockMap: make(map[T]*sync.Mutex)}
}

// Lock fetches the existing lock, or creates a new lock for the given ID, and locks it.
func (l *IDLock[T]) Lock(key T) {
	l.accessLock.Lock()

	lock, ok := l.lockMap[key]
	if !ok {
		lock = &sync.Mutex{}
		l.lockMap[key] = lock
	}

	l.accessLock.Unlock()

	lock.Lock()
}

// Unlock fetches the existing lock and deletes it from the map, before unlocking it.
func (l *IDLock[T]) Unlock(key T) {
	l.accessLock.Lock()
	defer l.accessLock.Unlock()

	lock, ok := l.lockMap[key]
	if !ok {
		return
	}

	lock.Unlock()
	delete(l.lockMap, key)
}
