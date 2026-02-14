package parallel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"coderaft/internal/security"
)

type WorkerPool struct {
	maxWorkers int
	timeout    time.Duration
}

func NewWorkerPool(maxWorkers int, timeout time.Duration) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = security.Limits.DefaultWorkerCount
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &WorkerPool{
		maxWorkers: maxWorkers,
		timeout:    timeout,
	}
}

type Task func() error

type Result struct {
	Index int
	Error error
}

func (wp *WorkerPool) Execute(tasks []Task) []error {
	if len(tasks) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), wp.timeout)
	defer cancel()

	taskChan := make(chan int, len(tasks))
	resultChan := make(chan Result, len(tasks))

	var wg sync.WaitGroup
	workerCount := wp.maxWorkers
	if len(tasks) < workerCount {
		workerCount = len(tasks)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case taskIndex, ok := <-taskChan:
					if !ok {
						return
					}
					err := tasks[taskIndex]()
					resultChan <- Result{Index: taskIndex, Error: err}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(taskChan)
		for i := range tasks {
			select {
			case taskChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	results := make([]error, len(tasks))
	for i := 0; i < len(tasks); i++ {
		select {
		case result := <-resultChan:
			results[result.Index] = result.Error
		case <-ctx.Done():
			results[i] = fmt.Errorf("task execution timeout")
		}
	}

	wg.Wait()
	close(resultChan)

	return results
}

type StringTask func() (string, error)

type StringResult struct {
	Index int
	Value string
	Error error
}

func (wp *WorkerPool) ExecuteStringTasks(tasks []StringTask) ([]string, []error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), wp.timeout)
	defer cancel()

	taskChan := make(chan int, len(tasks))
	resultChan := make(chan StringResult, len(tasks))

	var wg sync.WaitGroup
	workerCount := wp.maxWorkers
	if len(tasks) < workerCount {
		workerCount = len(tasks)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case taskIndex, ok := <-taskChan:
					if !ok {
						return
					}
					value, err := tasks[taskIndex]()
					resultChan <- StringResult{Index: taskIndex, Value: value, Error: err}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(taskChan)
		for i := range tasks {
			select {
			case taskChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	values := make([]string, len(tasks))
	errors := make([]error, len(tasks))
	for i := 0; i < len(tasks); i++ {
		select {
		case result := <-resultChan:
			values[result.Index] = result.Value
			errors[result.Index] = result.Error
		case <-ctx.Done():
			errors[i] = fmt.Errorf("task execution timeout")
		}
	}

	wg.Wait()
	close(resultChan)

	return values, errors
}

type Batch struct {
	Name  string
	Tasks []Task
}

func (wp *WorkerPool) ExecuteBatches(batches []Batch) map[string][]error {
	if len(batches) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	results := make(map[string][]error)
	resultsMux := sync.Mutex{}

	for _, batch := range batches {
		wg.Add(1)
		go func(b Batch) {
			defer wg.Done()
			batchResults := wp.Execute(b.Tasks)

			resultsMux.Lock()
			results[b.Name] = batchResults
			resultsMux.Unlock()
		}(batch)
	}

	wg.Wait()
	return results
}
