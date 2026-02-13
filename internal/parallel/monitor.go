package parallel

import (
	"fmt"
	"sync"
	"time"

	"coderaft/internal/ui"
)

type PerformanceMonitor struct {
	mu         sync.Mutex
	startTimes map[string]time.Time
	durations  map[string]time.Duration
}

func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		startTimes: make(map[string]time.Time),
		durations:  make(map[string]time.Duration),
	}
}

func (pm *PerformanceMonitor) Start(operation string) {
	pm.mu.Lock()
	pm.startTimes[operation] = time.Now()
	pm.mu.Unlock()
	ui.Status("starting: %s", operation)
}

func (pm *PerformanceMonitor) End(operation string) time.Duration {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if startTime, exists := pm.startTimes[operation]; exists {
		duration := time.Since(startTime)
		pm.durations[operation] = duration
		ui.Success("completed: %s in %v", operation, duration)
		delete(pm.startTimes, operation)
		return duration
	}
	return 0
}

func (pm *PerformanceMonitor) GetDuration(operation string) time.Duration {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.durations[operation]
}

func (pm *PerformanceMonitor) PrintSummary() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if len(pm.durations) == 0 {
		return
	}

	ui.Blank()
	ui.Header("performance summary")
	fmt.Printf("%-30s %s\n", "Operation", "Duration")
	fmt.Printf("%-30s %s\n", "----------", "--------")

	var total time.Duration
	for operation, duration := range pm.durations {
		fmt.Printf("%-30s %v\n", operation, duration)
		total += duration
	}

	fmt.Printf("%-30s %s\n", "----------", "--------")
	fmt.Printf("%-30s %v\n", "Total Time", total)
	ui.Blank()
}

func (pm *PerformanceMonitor) OperationTimer(operation string) func() {
	pm.Start(operation)
	return func() {
		pm.End(operation)
	}
}

func (pm *PerformanceMonitor) TimedOperation(operation string, fn func() error) error {
	defer pm.OperationTimer(operation)()
	return fn()
}
