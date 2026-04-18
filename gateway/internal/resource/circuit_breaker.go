package resource

import (
	"log"
	"sync/atomic"
	"time"
)

const (
	FDWarningRatio = 0.70 // 70% warning
	FDDangerRatio  = 0.85 // 85% danger - reject new connections
)

type CircuitState int

const (
	CircuitNormal CircuitState = iota
	CircuitWarning
	CircuitOpen
)

type CircuitBreaker struct {
	state         atomic.Int32
	fdLimit       uint64
	currentFD     atomic.Int64
	warningCount  atomic.Int32
	openUntil     atomic.Int64
	checkInterval time.Duration
	stopChan      chan struct{}
}

func NewCircuitBreaker(fdLimit uint64) *CircuitBreaker {
	return &CircuitBreaker{
		fdLimit:      fdLimit,
		checkInterval: 5 * time.Second,
		stopChan:     make(chan struct{}),
	}
}

func (cb *CircuitBreaker) Start() {
	go cb.monitorLoop()
}

func (cb *CircuitBreaker) Stop() {
	close(cb.stopChan)
}

func (cb *CircuitBreaker) monitorLoop() {
	ticker := time.NewTicker(cb.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cb.checkAndUpdate()
		case <-cb.stopChan:
			return
		}
	}
}

func (cb *CircuitBreaker) checkAndUpdate() {
	current, err := GetCurrentFDUsage()
	if err != nil {
		log.Printf("Failed to get FD usage: %v", err)
		return
	}

	cb.currentFD.Store(int64(current))
	usage := float64(current) / float64(cb.fdLimit)

	if usage >= FDDangerRatio {
		cb.state.Store(int32(CircuitOpen))
		cb.warningCount.Store(0)
		log.Printf("Circuit breaker OPEN: FD usage %d/%d (%.1f%%)", current, cb.fdLimit, usage*100)
	} else if usage >= FDWarningRatio {
		cb.state.Store(int32(CircuitWarning))
		cb.warningCount.Add(1)
		log.Printf("Circuit breaker WARNING: FD usage %d/%d (%.1f%%)", current, cb.fdLimit, usage*100)
	} else {
		cb.state.Store(int32(CircuitNormal))
		cb.warningCount.Store(0)
	}
}

func (cb *CircuitBreaker) AllowConnection() bool {
	state := CircuitState(cb.state.Load())

	if state == CircuitOpen {
		// Check if we should try half-open
		openUntil := cb.openUntil.Load()
		if openUntil > 0 && time.Now().UnixNano() > openUntil {
			// Try half-open
			return true
		}
		return false
	}

	return true
}

func (cb *CircuitBreaker) RecordConnection() {
	// Could increment connection counter
}

func (cb *CircuitBreaker) RecordClose() {
	// Could decrement connection counter
}

func (cb *CircuitBreaker) GetState() CircuitState {
	return CircuitState(cb.state.Load())
}

func (cb *CircuitBreaker) GetCurrentFDUsage() int64 {
	return cb.currentFD.Load()
}
