package main

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

// CPU密集型任务 - 计算素数
func cpuIntensiveTask(goroutineID int, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < iterations; i++ {
		fmt.Printf("Goroutine %d: iteration %d\n", goroutineID, i)
		
		// 计算大量素数
		primeCount := 0
		for n := 2; n < 10000; n++ {
			isPrime := true
			for j := 2; j <= int(math.Sqrt(float64(n))); j++ {
				if n%j == 0 {
					isPrime = false
					break
				}
			}
			if isPrime {
				primeCount++
			}
		}
		
		// 矩阵乘法计算
		matrixSize := 100
		a := make([][]float64, matrixSize)
		b := make([][]float64, matrixSize)
		c := make([][]float64, matrixSize)
		
		for i := 0; i < matrixSize; i++ {
			a[i] = make([]float64, matrixSize)
			b[i] = make([]float64, matrixSize)
			c[i] = make([]float64, matrixSize)
			for j := 0; j < matrixSize; j++ {
				a[i][j] = float64(i + j)
				b[i][j] = float64(i * j)
			}
		}
		
		// 执行矩阵乘法
		for i := 0; i < matrixSize; i++ {
			for j := 0; j < matrixSize; j++ {
				for k := 0; k < matrixSize; k++ {
					c[i][j] += a[i][k] * b[k][j]
				}
			}
		}
		
		// 短暂休息
		time.Sleep(10 * time.Millisecond)
	}
}

func main() {
	fmt.Println("Starting CPU-intensive test program for on-CPU profiling...")
	fmt.Println("This program will run for about 5 minutes. Use Ctrl+C to stop.")
	
	// 设置使用所有可用的CPU核心
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	var wg sync.WaitGroup
	
	// 启动多个goroutine进行CPU密集型计算
	numGoroutines := 6
	iterationsPerGoroutine := 500
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go cpuIntensiveTask(i, iterationsPerGoroutine, &wg)
	}
	
	wg.Wait()
	fmt.Println("Test program completed.")
}