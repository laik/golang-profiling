package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"
)

// 模拟CPU密集型计算
func cpuIntensiveTask(n int) int {
	result := 0
	for i := 0; i < n; i++ {
		result += i * i
	}
	return result
}

// 模拟内存分配
func memoryIntensiveTask() {
	data := make([][]int, 1000)
	for i := range data {
		data[i] = make([]int, 1000)
		for j := range data[i] {
			data[i][j] = rand.Intn(100)
		}
	}
	// 强制垃圾回收
	runtime.GC()
}

// 递归函数
func recursiveFunction(depth int) int {
	if depth <= 0 {
		return 1
	}
	return depth * recursiveFunction(depth-1)
}

// 模拟网络延迟
func simulateNetworkDelay() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
}

func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		fmt.Printf("Worker %d processing job %d\n", id, j)

		// 执行不同类型的任务
		switch j % 4 {
		case 0:
			cpuIntensiveTask(100000)
		case 1:
			memoryIntensiveTask()
		case 2:
			recursiveFunction(10)
		case 3:
			simulateNetworkDelay()
		}

		results <- j * 2
	}
}

func main() {
	fmt.Println("Starting Go profiling test program...")

	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	// 打印程序信息
	fmt.Printf("PID: %d\n", os.Getpid())
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))

	// 创建工作池
	const numWorkers = 3
	const numJobs = 20

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)

	// 启动workers
	for w := 1; w <= numWorkers; w++ {
		go worker(w, jobs, results)
	}

	// 发送任务
	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}
	close(jobs)

	// 收集结果
	for a := 1; a <= numJobs; a++ {
		<-results
	}

	fmt.Println("All jobs completed. Program will run for 30 seconds for profiling...")

	// 持续运行以便进行profiling
	fmt.Println("Running continuous workload for profiling...")
	for {
		// 混合不同类型的负载
		go cpuIntensiveTask(50000)
		go memoryIntensiveTask()
		go func() {
			recursiveFunction(8)
		}()

		// 主线程也执行一些任务
		cpuIntensiveTask(30000)
		memoryIntensiveTask()

		// 短暂休息，避免CPU占用过高
		time.Sleep(10 * time.Millisecond)
	}
}
