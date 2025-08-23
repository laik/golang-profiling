package main

import (
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// CPU密集型任务
func cpuIntensiveTask(goroutineID int, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < iterations; i++ {
		fmt.Printf("CPU Goroutine %d: iteration %d\n", goroutineID, i)
		
		// 计算素数
		primeCount := 0
		for n := 2; n < 5000; n++ {
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
		
		// 短暂休息
		time.Sleep(50 * time.Millisecond)
	}
}

// I/O等待任务
func ioWaitTask(goroutineID int, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < iterations; i++ {
		fmt.Printf("IO Goroutine %d: iteration %d\n", goroutineID, i)
		
		// 模拟文件I/O等待
		time.Sleep(200 * time.Millisecond)
		
		// 一些CPU计算
		sum := 0
		for j := 0; j < 100000; j++ {
			sum += j
		}
		
		// 再次等待
		time.Sleep(100 * time.Millisecond)
	}
}

// 网络等待任务
func networkWaitTask(goroutineID int, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	for i := 0; i < iterations; i++ {
		fmt.Printf("Network Goroutine %d: iteration %d\n", goroutineID, i)
		
		// 模拟网络请求（会失败，但会产生等待）
		_, err := client.Get("http://127.0.0.1:9999/nonexistent")
		if err != nil {
			// 预期的错误，忽略
		}
		
		// 一些CPU计算
		result := 1.0
		for j := 0; j < 50000; j++ {
			result *= 1.0001
		}
		
		// 短暂休息
		time.Sleep(150 * time.Millisecond)
	}
}

// 混合任务 - 既有CPU密集型又有等待
func mixedTask(goroutineID int, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < iterations; i++ {
		fmt.Printf("Mixed Goroutine %d: iteration %d\n", goroutineID, i)
		
		// CPU密集型计算
		matrixSize := 50
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
		
		// 矩阵乘法
		for i := 0; i < matrixSize; i++ {
			for j := 0; j < matrixSize; j++ {
				for k := 0; k < matrixSize; k++ {
					c[i][j] += a[i][k] * b[k][j]
				}
			}
		}
		
		// I/O等待
		time.Sleep(300 * time.Millisecond)
		
		// 更多CPU计算
		sum := 0.0
		for j := 0; j < 100000; j++ {
			sum += math.Sin(float64(j))
		}
		
		// 短暂休息
		time.Sleep(100 * time.Millisecond)
	}
}

func main() {
	fmt.Println("Starting mixed workload test program for both on-CPU and off-CPU profiling...")
	fmt.Println("This program will run for about 5 minutes. Use Ctrl+C to stop.")
	
	// 设置使用所有可用的CPU核心
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	var wg sync.WaitGroup
	
	// 启动不同类型的goroutine
	numIterations := 150
	
	// 2个CPU密集型goroutine
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go cpuIntensiveTask(i, numIterations, &wg)
	}
	
	// 2个I/O等待goroutine
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go ioWaitTask(i+10, numIterations, &wg)
	}
	
	// 2个网络等待goroutine
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go networkWaitTask(i+20, numIterations, &wg)
	}
	
	// 2个混合任务goroutine
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go mixedTask(i+30, numIterations, &wg)
	}
	
	wg.Wait()
	fmt.Println("Mixed workload test program completed.")
}