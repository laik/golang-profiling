# 更新日志

## [v0.3.0] - 2025-01-21

### 新增功能
- **Off-CPU 分析支持**
  - 实现了完整的 off-CPU 性能分析功能
  - 添加了 `sched_switch` tracepoint 处理，用于捕获进程调度事件
  - 支持同时收集 on-CPU 和 off-CPU 数据的混合分析模式
  - 新增时间戳记录和持续时间计算功能

### 数据结构优化
- **扩展 eBPF 数据结构**
  - 在 `EbpfProfileKey` 中添加了 `sample_type` 字段用于区分 on-CPU 和 off-CPU 事件
  - 添加了内存对齐填充 `_padding: [0; 3]` 确保结构体在不同平台上的一致性
  - 优化了数据聚合逻辑以支持多种采样类型

### 测试和验证
- **完善测试程序**
  - 创建了 `test_offcpu.go` 专门测试 off-CPU 分析功能
  - 创建了 `test_mixed.go` 测试混合分析模式
  - 验证了火焰图生成的准确性和完整性

### 技术细节
- **eBPF 程序增强**
  - 实现了 `sched_switch` tracepoint 钩子函数
  - 添加了进程状态跟踪和时间戳管理
  - 优化了堆栈收集逻辑以支持不同的采样场景

### 问题分析和解决
- **火焰图显示问题**
  - 分析了采样频率对函数显示的影响（默认 99 Hz）
  - 识别了 off-CPU 时间较长的函数可能不会在 on-CPU 火焰图中显示的原因
  - 提供了提高采样频率和延长分析时间的优化建议

### 内存对齐原理说明
- **eBPF 结构体对齐**
  - 详细解释了 `_padding` 字段在 eBPF 程序中的重要性
  - 阐述了内存对齐对硬件访问效率、eBPF 验证器要求和跨平台兼容性的影响
  - 强调了作为 HashMap key 时哈希一致性的重要性

---

## [v0.2.0] - 2025-08-22

### 新增功能
- **PID 过滤支持**
  - 添加了精确的 PID 过滤功能，支持对指定进程进行性能分析
  - 优化了 eBPF map 结构以提升性能

### 修复问题
- **eBPF 程序触发问题**
  - 修复了空闲进程（如 etcd）不触发 perf 事件的问题
  - 为服务器应用程序添加了适当的活动检测

### 变更
- **Map 结构优化**
  - 将 `TARGET_PID` 从 HashMap 改为 Array 以提升性能
  - 减少内存使用：`STACK_TRACES` (16384→8192)，`COUNTS` (40960→16384)

### 技术细节
- **根本原因分析**
  - 空闲状态的服务器进程很少触发 CPU 密集型操作
  - eBPF 程序依赖 perf 事件来收集堆栈信息
  - 需要活跃的工作负载才能收集到有意义的性能数据

---

## [v0.1.0] - 2025-08-18

### 修复问题
- **Go 符号解析问题**
  - 修复了被 strip 处理的 Go 二进制文件的符号解析问题
  - 修正了 `.gopclntab` 解析中的 PC 地址转换逻辑

### 技术背景

#### 问题描述
在使用 eBPF 对 Go 程序进行性能分析时，发现即使 Go 程序被 `strip` 处理后仍包含 `.gopclntab` 段，但生成的火焰图只显示十六进制地址（如 `[unknown:0x4948c5]`），而不是可读的函数名。

#### 根本原因
问题由 PC 地址处理中的**双重地址转换**导致：

1. **第一次转换**在 `parse_pclntab_enhanced` 中：将相对 PC 转换为绝对 PC
2. **第二次转换**在 `resolve_pc` 中：错误地将绝对 PC 又转换回相对 PC

#### 解决方案
移除了 `resolve_pc` 方法中错误的地址转换逻辑：

```rust
// 修复前（错误）
let lookup_pc = if gopclntab.version >= 18 {
    pc - gopclntab.text_start  // 错误！
} else {
    pc - base_addr            // 错误！
};

// 修复后（正确）
let lookup_pc = pc;  // 直接使用绝对地址
```

#### 修复结果

**修复前：**
```
[unknown:0x4948c5] (2 samples, 0.63%)
[unknown:0x46e1a1] (1 samples, 0.31%)
[unknown:0x4415d8] (1 samples, 0.31%)
```

**修复后：**
```
runtime.tgkill /usr/local/go/src/runtime/sys_linux_amd64.s:177
runtime.preemptone /usr/local/go/src/runtime/proc.go:6374
main::main /root/withlin/golang-profile/example/main.go:117
math::rand.(Rand).Intn /usr/local/go/src/math/rand/rand.go:183
```

### 关键技术要点

#### Go 二进制结构
Go 程序包含一个特殊的 `.gopclntab`（Go 程序计数器行表）段，包含：

- **函数表 (functab)**：存储每个函数的 PC 地址和元数据偏移
- **函数名表 (funcnametab)**：存储所有函数名字符串
- **文件表 (filetab)**：存储源文件路径
- **PC表 (pctab)**：存储 PC 到行号的映射关系

#### 版本兼容性
- **Go 1.2-1.16**：PC 地址相对于二进制文件基地址
- **Go 1.18+**：引入了 `text_start` 字段，PC 地址相对于 text 段起始地址

### 修改文件
- `golang-profiling/src/symbol_resolver.rs`
  - 修复了 `resolve_pc` 方法中的地址转换逻辑
  - 简化了 `parse_pclntab_enhanced` 中的函数表构建

### 影响
此修复显著提升了 Go 程序性能分析的可读性和实用性，即使对于被 strip 处理的二进制文件也能正确解析符号。

---

## 开发说明

### 测试
- 使用 `test_app_active`（strip 后的二进制）进行验证
- 支持 Go 1.18+ 和早期版本
- 火焰图现在正确显示函数名和源码位置

### 最佳实践
1. **理解数据结构**：深入理解 `.gopclntab` 的内部结构
2. **地址空间管理**：保持地址转换的一致性
3. **版本兼容性**：考虑不同 Go 版本的差异
4. **调试驱动开发**：使用详细日志辅助问题定位