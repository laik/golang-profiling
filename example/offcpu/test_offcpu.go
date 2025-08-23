package main

import (
	"fmt"
	"sync"
	"time"
)

func cpuIntensiveTask() {
	// CPU-intensive task
	for i := 0; i < 50000000; i++ {
		_ = i * i
	}
}

func ioWaitTask() {
	// Simulate I/O wait (off-CPU)
	time.Sleep(200 * time.Millisecond)
}

func networkWaitTask() {
	// Simulate network wait
	time.Sleep(100 * time.Millisecond)
}

func main() {
	fmt.Println("Starting long-running test program for off-CPU profiling...")
	fmt.Println("This program will run for about 5 minutes. Use Ctrl+C to stop.")

	var wg sync.WaitGroup

	// Start multiple goroutines with different behaviors
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Run for about 5 minutes
			for j := 0; j < 250; j++ {
				fmt.Printf("Goroutine %d: iteration %d\n", id, j)

				// Mix of CPU and I/O operations
				cpuIntensiveTask()
				ioWaitTask()
				networkWaitTask()

				// Add some variety
				if j%3 == 0 {
					time.Sleep(500 * time.Millisecond) // Longer sleep
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("Test program completed.")
}
