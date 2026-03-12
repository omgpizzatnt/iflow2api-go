package handlers

import (
	"sync"

	"iflow2api-go/internal/logger"
)

var (
	apiRequestLock *semaphoreWrapper
)

type semaphoreWrapper struct {
	semaphore chan struct{}
	waiting   int
	running   int
	mu        sync.Mutex
}

func initSemaphore(maxConcurrent int) {
	if apiRequestLock == nil {
		apiRequestLock = &semaphoreWrapper{
			semaphore: make(chan struct{}, maxConcurrent),
		}
		logger.Printf("Concurrency control initialized: max_concurrent=%d", maxConcurrent)
	} else {
		logger.Printf("Concurrency control already initialized")
	}
}

func AcquireLock() {
	if apiRequestLock == nil {
		initSemaphore(1)
	}

	apiRequestLock.mu.Lock()
	apiRequestLock.waiting++
	apiRequestLock.mu.Unlock()

	apiRequestLock.semaphore <- struct{}{}

	apiRequestLock.mu.Lock()
	apiRequestLock.waiting--
	apiRequestLock.running++
	logger.Debugf("Acquired lock: running=%d, waiting=%d", apiRequestLock.running, apiRequestLock.waiting)
	apiRequestLock.mu.Unlock()
}

func ReleaseLock() {
	if apiRequestLock == nil {
		return
	}

	<-apiRequestLock.semaphore

	apiRequestLock.mu.Lock()
	apiRequestLock.running--
	logger.Debugf("Released lock: running=%d, waiting=%d", apiRequestLock.running, apiRequestLock.waiting)
	apiRequestLock.mu.Unlock()
}

func SetMaxConcurrency(max int) {
	if apiRequestLock != nil {
		logger.Printf("Warning: Concurrency control already initialized, cannot change")
		return
	}
	initSemaphore(max)
}
