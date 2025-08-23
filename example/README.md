# Golang Profiling Examples

本目录包含用于测试 Golang eBPF 性能分析工具的示例程序。

## 目录结构

```
example/
├── README.md           # 本说明文件
├── basic/              # 基础示例
│   ├── go.mod         # Go 模块文件
│   └── main.go        # 基础示例程序
├── executables/        # 编译后的可执行文件
│   ├── test_app_active # 基础测试程序
│   ├── test_mixed     # 混合负载测试程序
│   ├── test_offcpu    # off-CPU 测试程序
│   └── test_oncpu     # on-CPU 测试程序
├── mixed/              # 混合负载测试
│   └── test_mixed.go  # 同时包含 CPU 密集型和 I/O 等待的测试程序
├── offcpu/             # off-CPU 分析测试
│   ├── test_offcpu    # 可执行文件（已移动到 executables/）
│   └── test_offcpu.go # 包含 I/O 等待、网络等待等阻塞操作的测试程序
└── oncpu/              # on-CPU 分析测试
    ├── test_oncpu     # 可执行文件（已移动到 executables/）
    └── test_oncpu.go  # CPU 密集型计算测试程序
```

## 测试程序说明

### 1. test_oncpu - on-CPU 分析测试
- **用途**: 测试 on-CPU 性能分析功能
- **特点**: CPU 密集型计算（素数计算、矩阵乘法）
- **运行时间**: 约 5 分钟
- **启动方式**: `./executables/test_oncpu`

### 2. test_offcpu - off-CPU 分析测试
- **用途**: 测试 off-CPU 性能分析功能
- **特点**: 包含 I/O 等待、网络等待、sleep 等阻塞操作
- **运行时间**: 约 5 分钟
- **启动方式**: `./executables/test_offcpu`

### 3. test_mixed - 混合负载测试
- **用途**: 同时测试 on-CPU 和 off-CPU 分析功能
- **特点**: 包含多种类型的 goroutine：
  - CPU 密集型任务
  - I/O 等待任务
  - 网络等待任务
  - 混合任务（CPU + I/O）
- **运行时间**: 约 5 分钟
- **启动方式**: `./executables/test_mixed`

### 4. test_app_active - 基础测试程序
- **用途**: 基础的活跃程序测试
- **启动方式**: `./executables/test_app_active`

## 使用方法

### 1. 编译测试程序
```bash
# 编译 on-CPU 测试程序
cd oncpu && go build -o ../executables/test_oncpu test_oncpu.go

# 编译 off-CPU 测试程序
cd offcpu && go build -o ../executables/test_offcpu test_offcpu.go

# 编译混合测试程序
cd mixed && go build -o ../executables/test_mixed test_mixed.go
```

### 2. 运行性能分析

#### on-CPU 分析
```bash
# 启动测试程序
./executables/test_oncpu &
TEST_PID=$!

# 进行 on-CPU 分析（默认模式）
cd .. && cargo run -- --pid $TEST_PID --duration 30 --output oncpu_flamegraph.svg
```

#### off-CPU 分析
```bash
# 启动测试程序
./executables/test_offcpu &
TEST_PID=$!

# 进行 off-CPU 分析
cd .. && cargo run -- --pid $TEST_PID --duration 30 --off-cpu --output offcpu_flamegraph.svg
```

#### 混合分析
```bash
# 启动混合测试程序
./executables/test_mixed &
TEST_PID=$!

# 进行分析（会同时收集 on-CPU 和 off-CPU 数据）
cd .. && cargo run -- --pid $TEST_PID --duration 30 --output mixed_flamegraph.svg
```

## 注意事项

1. 所有测试程序现在都设计为运行约 5 分钟，提供足够的时间进行性能分析
2. 可以使用 `Ctrl+C` 提前停止测试程序
3. 建议在分析期间让测试程序持续运行，以获得更准确的性能数据
4. 火焰图文件将保存在项目根目录中