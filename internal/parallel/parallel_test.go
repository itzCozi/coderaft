package parallel

import (
	"fmt"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {

	pool := NewWorkerPool(2, 5*time.Second)

	tasks := []Task{
		func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
		func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
		func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}

	start := time.Now()
	results := pool.Execute(tasks)
	duration := time.Since(start)

	if duration > 300*time.Millisecond {
		t.Errorf("Expected parallel execution to be faster, took %v", duration)
	}

	for i, err := range results {
		if err != nil {
			t.Errorf("Task %d failed: %v", i, err)
		}
	}
}

func TestStringTasks(t *testing.T) {
	pool := NewWorkerPool(3, 5*time.Second)

	tasks := []StringTask{
		func() (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "result1", nil
		},
		func() (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "result2", nil
		},
	}

	values, errors := pool.ExecuteStringTasks(tasks)

	if len(values) != 2 || len(errors) != 2 {
		t.Error("Expected 2 results")
	}

	if values[0] != "result1" || values[1] != "result2" {
		t.Error("Unexpected results")
	}

	for i, err := range errors {
		if err != nil {
			t.Errorf("Task %d failed: %v", i, err)
		}
	}
}

func TestSetupCommandExecutor(t *testing.T) {

	executor := NewSetupCommandExecutor("test-island", false, 2)

	commands := []string{
		"apt install -y git",
		"pip install flask",
		"npm install -g typescript",
		"yarn global add webpack",
		"systemctl start nginx",
	}

	groups := executor.categorizeCommands(commands)

	if len(groups) == 0 {
		t.Error("Expected command groups to be created")
	}

	foundSystemGroup := false
	for _, group := range groups {
		if group.Name == "System Commands" {
			foundSystemGroup = true
			if group.Parallel {
				t.Error("System commands should not be parallel")
			}
		}
	}

	if !foundSystemGroup {
		t.Error("Expected system commands group to be created")
	}
}

func TestPerformanceMonitor(t *testing.T) {
	monitor := NewPerformanceMonitor()

	monitor.Start("test-operation")
	time.Sleep(100 * time.Millisecond)
	duration := monitor.End("test-operation")

	if duration < 100*time.Millisecond {
		t.Error("Duration should be at least 100ms")
	}

	err := monitor.TimedOperation("test-func", func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Error("Timed operation should not fail")
	}

	if monitor.GetDuration("test-func") < 50*time.Millisecond {
		t.Error("Function duration should be at least 50ms")
	}
}

func BenchmarkParallelExecution(b *testing.B) {
	pool := NewWorkerPool(4, 10*time.Second)

	for i := 0; i < b.N; i++ {
		tasks := []Task{
			func() error { time.Sleep(10 * time.Millisecond); return nil },
			func() error { time.Sleep(10 * time.Millisecond); return nil },
			func() error { time.Sleep(10 * time.Millisecond); return nil },
			func() error { time.Sleep(10 * time.Millisecond); return nil },
		}

		pool.Execute(tasks)
	}
}

func ExampleWorkerPool() {

	pool := NewWorkerPool(3, 1*time.Minute)

	tasks := []Task{
		func() error {
			fmt.Println("Task 1 executing")
			return nil
		},
		func() error {
			fmt.Println("Task 2 executing")
			return nil
		},
		func() error {
			fmt.Println("Task 3 executing")
			return nil
		},
	}

	results := pool.Execute(tasks)

	for i, err := range results {
		if err != nil {
			fmt.Printf("Task %d failed: %v\n", i, err)
		}
	}
}
