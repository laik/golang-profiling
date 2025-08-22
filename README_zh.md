# golang-profiling

[English](README.md) | 中文文档

[![CI](https://github.com/YOUR_USERNAME/golang-profile/workflows/CI/badge.svg)](https://github.com/YOUR_USERNAME/golang-profile/actions/workflows/ci.yml)
[![Security Audit](https://github.com/YOUR_USERNAME/golang-profile/workflows/Security%20Audit/badge.svg)](https://github.com/YOUR_USERNAME/golang-profile/actions/workflows/security.yml)
[![Documentation](https://github.com/YOUR_USERNAME/golang-profile/workflows/Documentation/badge.svg)](https://github.com/YOUR_USERNAME/golang-profile/actions/workflows/docs.yml)
[![License](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue.svg)](LICENSE-MIT)

高性能的 Golang CPU 性能分析工具，基于 eBPF 技术实现，支持生成火焰图进行性能可视化分析。

## 🔥 示例火焰图

![示例火焰图](example_flamegraph.svg)

*交互式火焰图展示 CPU 性能分析结果，内核函数（绿色）和用户态函数（蓝色）*

## 项目简介

`golang-profile` 是一个专为 Golang 应用程序设计的 CPU 性能分析工具。它使用 eBPF (Extended Berkeley Packet Filter) 技术在内核层面收集性能数据，具有极低的性能开销，能够在生产环境中安全使用。

### 主要特性

- 🚀 **高性能**: 基于 eBPF 技术，性能开销极低
- 🔥 **火焰图生成**: 支持生成交互式火焰图，直观展示性能热点
- 🎯 **精确分析**: 支持按进程 PID 或进程名称进行精确分析
- ⚙️ **灵活配置**: 丰富的命令行参数，支持自定义分析参数
- 📊 **多种输出格式**: 支持 SVG 火焰图和折叠堆栈格式
- 🎨 **自定义样式**: 支持多种颜色主题和火焰图样式定制

## 系统要求

- **操作系统**: Linux (内核版本 4.4+)
- **权限**: 需要 root 权限或 CAP_BPF 能力
- **依赖**: Perl (用于火焰图生成)

## 安装方式

### 方式一：下载预编译二进制文件

从 [GitHub Releases](https://github.com/YOUR_USERNAME/golang-profile/releases) 下载最新版本：

```bash
# 下载 x86_64 Linux 版本
wget https://github.com/YOUR_USERNAME/golang-profile/releases/latest/download/golang-profiling-linux-x86_64.tar.gz
tar -xzf golang-profiling-linux-x86_64.tar.gz
sudo mv golang-profiling /usr/local/bin/

# 下载 ARM64 Linux 版本
wget https://github.com/YOUR_USERNAME/golang-profile/releases/latest/download/golang-profiling-linux-aarch64.tar.gz
tar -xzf golang-profiling-linux-aarch64.tar.gz
sudo mv golang-profiling /usr/local/bin/
```

### 方式二：从源码编译

#### 1. 安装 Rust 工具链

```bash
# 安装 Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# 安装 nightly 工具链 (eBPF 开发需要)
rustup install nightly
rustup component add rust-src --toolchain nightly
```

### 2. 安装系统依赖

#### Ubuntu/Debian
```bash
sudo apt update
sudo apt install -y \
    build-essential \
    pkg-config \
    libbpf-dev \
    libelf-dev \
    zlib1g-dev \
    perl
```

#### CentOS/RHEL/Fedora
```bash
# CentOS/RHEL
sudo yum install -y \
    gcc \
    pkg-config \
    libbpf-devel \
    elfutils-libelf-devel \
    zlib-devel \
    perl

# Fedora
sudo dnf install -y \
    gcc \
    pkg-config \
    libbpf-devel \
    elfutils-libelf-devel \
    zlib-devel \
    perl
```

### 3. 安装 bpf-linker

```bash
cargo install bpf-linker
```

## 编译项目

```bash
# 克隆项目
git clone <repository-url>
cd golang-profile

# 编译项目
cargo build --release

# 或者直接运行 (debug 模式)
cargo build
```

## 使用方法

### 基本用法

```bash
# 分析指定 PID 的进程，持续 10 秒
sudo ./target/release/golang-profiling --pid 1234 --duration 10

# 分析指定名称的进程
sudo ./target/release/golang-profiling --process-name "my-go-app" --duration 5

# 自定义输出文件名
sudo ./target/release/golang-profiling --pid 1234 --output my-profile.svg
```

### 完整参数说明

| 参数 | 短参数 | 默认值 | 说明 |
|------|--------|--------|------|
| `--pid` | `-p` | - | 目标进程的 PID |
| `--process-name` | `-n` | - | 目标进程的名称 |
| `--duration` | `-d` | 5 | 分析持续时间（秒） |
| `--output` | `-o` | flamegraph.svg | 输出文件路径 |
| `--frequency` | `-f` | 99 | 采样频率（Hz） |
| `--off-cpu` | - | false | 启用 off-CPU 分析 |
| `--verbose` | `-v` | false | 详细输出模式 |
| `--export-folded` | - | - | 导出折叠堆栈格式文件 |

### 火焰图自定义参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--title` | "Golang CPU Profile" | 火焰图标题 |
| `--subtitle` | - | 火焰图副标题 |
| `--colors` | "hot" | 颜色主题 (hot/mem/io/java/js/perl/red/green/blue) |
| `--bgcolors` | - | 背景颜色 (yellow/blue/green/grey 或 #rrggbb) |
| `--width` | 1200 | 图像宽度（像素） |
| `--height` | 16 | 每个框架高度（像素） |
| `--fonttype` | "Verdana" | 字体类型 |
| `--fontsize` | 12 | 字体大小 |
| `--inverted` | false | 生成倒置火焰图（冰柱图） |
| `--flamechart` | false | 生成火焰图表（按时间排序） |
| `--hash` | false | 使用函数名哈希着色 |
| `--random` | false | 随机颜色生成 |

## 使用示例

### 1. 基础性能分析

```bash
# 分析 PID 为 1234 的进程，持续 30 秒
sudo ./target/release/golang-profiling \
    --pid 1234 \
    --duration 30 \
    --output app-profile.svg
```

### 2. 自定义火焰图样式

```bash
# 生成带有自定义标题和颜色的火焰图
sudo ./target/release/golang-profiling \
    --pid 1234 \
    --duration 10 \
    --title "我的应用性能分析" \
    --subtitle "生产环境 - 2024年" \
    --colors java \
    --width 1600 \
    --height 20 \
    --fontsize 14 \
    --output custom-profile.svg
```

### 3. 按进程名称分析

```bash
# 分析名为 "my-go-service" 的进程
sudo ./target/release/golang-profiling \
    --process-name "my-go-service" \
    --duration 15 \
    --frequency 199 \
    --verbose
```

### 4. 导出原始数据

```bash
# 同时生成火焰图和导出折叠堆栈数据
sudo ./target/release/golang-profiling \
    --pid 1234 \
    --duration 10 \
    --output profile.svg \
    --export-folded profile.folded
```

### 5. 生成倒置火焰图（冰柱图）

```bash
# 生成冰柱图，适合分析调用链
sudo ./target/release/golang-profiling \
    --pid 1234 \
    --duration 10 \
    --inverted \
    --title "调用链分析" \
    --output icicle.svg
```

## 输出文件说明

- **SVG 火焰图**: 交互式的火焰图，可以在浏览器中打开，支持点击缩放和搜索
- **折叠堆栈文件**: 文本格式的原始数据，可用于其他分析工具

### 示例火焰图

以下是使用本工具生成的火焰图示例：

![示例火焰图](example_flamegraph.svg)

> 💡 **提示**: 点击上面的火焰图可以查看完整的交互式版本，支持缩放和搜索功能。

## 性能分析技巧

1. **选择合适的采样频率**: 
   - 高频率 (199Hz+): 适合短时间精确分析
   - 低频率 (49Hz-99Hz): 适合长时间监控

2. **分析时长建议**:
   - 开发环境: 5-10 秒
   - 生产环境: 30-60 秒

3. **火焰图阅读**:
   - X 轴: 不代表时间，而是按字母顺序排列的函数
   - Y 轴: 调用栈深度
   - 宽度: 函数占用 CPU 时间的比例

## 故障排除

### 常见问题

1. **权限不足**
   ```
   Error: Permission denied
   ```
   解决方案: 使用 `sudo` 运行或确保用户有 CAP_BPF 权限

2. **找不到进程**
   ```
   Error: Process not found
   ```
   解决方案: 检查进程 PID 或名称是否正确

3. **eBPF 不支持**
   ```
   Error: BPF program load failed
   ```
   解决方案: 确保内核版本 >= 4.4 且支持 eBPF

## 许可证

本项目采用多重许可证:
- Apache License 2.0
- MIT License  
- GPL v2 License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 相关项目

- [FlameGraph](https://github.com/brendangregg/FlameGraph) - Brendan Gregg 的火焰图工具
- [aya](https://github.com/aya-rs/aya) - Rust eBPF 库
