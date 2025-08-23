# kubectl-pprof

一个用于在 Kubernetes 环境中对 Go 应用程序进行性能分析的 kubectl 插件。

## 功能特性

- 🎯 **精确定位**: 自动发现目标 Pod 和容器
- 🔍 **多种分析**: 支持 CPU、内存、goroutine、block、mutex 分析
- 🔥 **火焰图生成**: 自动生成 SVG/PNG/PDF 格式的火焰图
- 🚀 **Job 调度**: 使用 Kubernetes Job 进行分布式分析
- 🔒 **命名空间共享**: 支持 PID 命名空间共享
- 🐳 **容器运行时**: 支持 containerd、Docker、CRI-O
- ⚡ **高性能**: 优化的分析算法和资源使用
- 🛡️ **安全**: 最小权限原则和安全最佳实践

## 安装

### 从源码构建

```bash
git clone https://github.com/withlin/kubectl-pprof.git
cd kubectl-pprof
make build
sudo make install
```

### 使用 Go 安装

```bash
go install github.com/withlin/kubectl-pprof/cmd@latest
```

### 下载预编译二进制文件

从 [Releases](https://github.com/withlin/kubectl-pprof/releases) 页面下载适合您平台的二进制文件。

## 快速开始

### 基本用法

```bash
# 分析默认容器
kubectl pprof my-namespace my-pod

# 分析指定容器
kubectl pprof my-namespace my-pod my-container

# 指定分析时间和输出文件
kubectl pprof -d 60s -o /tmp/profile.svg my-namespace my-pod
```

### 高级用法

```bash
# 内存分析
kubectl pprof --type memory -d 30s my-namespace my-pod

# 使用自定义镜像
kubectl pprof -i my-registry/golang-profiling:v1.0 my-namespace my-pod

# 指定节点
kubectl pprof --node worker-node-1 my-namespace my-pod

# 生成 JSON 报告
kubectl pprof --json --format json -o report.json my-namespace my-pod
```

## 命令行选项

### 基础选项

| 选项 | 短选项 | 默认值 | 描述 |
|------|--------|--------|------|
| `--duration` | `-d` | `30s` | 分析持续时间 |
| `--output` | `-o` | `flamegraph.svg` | 输出文件路径 |
| `--image` | `-i` | `golang-profiling:latest` | 分析工具镜像 |
| `--node` | `-n` | `` | 强制在指定节点运行 |
| `--type` | | `cpu` | 分析类型 (cpu, memory, goroutine, block, mutex) |

### 输出选项

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--flamegraph` | `true` | 生成火焰图 |
| `--raw` | `false` | 保存原始分析数据 |
| `--json` | `false` | 生成 JSON 报告 |
| `--format` | `svg` | 输出格式 (svg, png, pdf, json) |

### 高级选项

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--sample-rate` | `0` | 采样率 (0 为默认) |
| `--stack-depth` | `0` | 最大栈深度 (0 为无限制) |
| `--filter` | `` | 函数名过滤模式 |
| `--ignore` | `` | 函数名忽略模式 |
| `--cpu-limit` | `1` | CPU 限制 |
| `--memory-limit` | `512Mi` | 内存限制 |
| `--timeout` | `5m` | Job 超时时间 |
| `--privileged` | `true` | 特权模式运行 |
| `--cleanup` | `true` | 完成后清理资源 |

## 工作原理

1. **目标发现**: 插件首先查找指定的 Pod 和容器
2. **节点定位**: 确定目标 Pod 运行的节点
3. **Job 创建**: 在目标节点创建分析 Job
4. **命名空间共享**: Job Pod 与目标 Pod 共享 PID 命名空间
5. **性能分析**: 使用 golang-profiling 工具进行分析
6. **结果收集**: 收集分析结果并生成火焰图
7. **资源清理**: 清理临时创建的 Job 资源

## 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   kubectl-pprof │───▶│  Kubernetes API │───▶│   Target Pod    │
│     Plugin      │    │     Server      │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       ▼                       │
         │              ┌─────────────────┐              │
         │              │  Profiling Job  │              │
         │              │                 │              │
         │              └─────────────────┘              │
         │                       │                       │
         │                       │ PID Namespace         │
         │                       │    Sharing            │
         │                       ▼                       │
         │              ┌─────────────────┐              │
         └─────────────▶│ golang-profiling│◀─────────────┘
                        │      Tool       │
                        └─────────────────┘
                                 │
                                 ▼
                        ┌─────────────────┐
                        │   Flame Graph   │
                        │     Output      │
                        └─────────────────┘
```

## 支持的分析类型

### CPU 分析
```bash
kubectl pprof --type cpu -d 30s my-namespace my-pod
```

### 内存分析
```bash
kubectl pprof --type memory -d 30s my-namespace my-pod
```

### Goroutine 分析
```bash
kubectl pprof --type goroutine my-namespace my-pod
```

### 阻塞分析
```bash
kubectl pprof --type block -d 30s my-namespace my-pod
```

### 互斥锁分析
```bash
kubectl pprof --type mutex -d 30s my-namespace my-pod
```

## 容器运行时支持

- **containerd**: 完全支持
- **Docker**: 完全支持
- **CRI-O**: 完全支持

## 权限要求

插件需要以下 Kubernetes 权限：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubectl-pprof
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "delete"]
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]
```

## 故障排除

### 常见问题

1. **权限不足**
   ```
   Error: failed to create job: forbidden
   ```
   解决方案：确保有足够的 RBAC 权限

2. **目标 Pod 不存在**
   ```
   Error: failed to find pod: not found
   ```
   解决方案：检查命名空间和 Pod 名称

3. **Job 超时**
   ```
   Error: job did not complete: context deadline exceeded
   ```
   解决方案：增加 `--timeout` 值

### 调试模式

```bash
# 启用详细日志
kubectl pprof --verbose my-namespace my-pod

# 保留 Job 资源用于调试
kubectl pprof --cleanup=false my-namespace my-pod
```

## 开发

### 构建

```bash
# 构建
make build

# 运行测试
make test

# 代码检查
make lint

# 交叉编译
make build-all
```

### 开发模式

```bash
# 启动开发模式（热重载）
make dev
```

### 测试

```bash
# 运行所有测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 运行基准测试
make bench
```

## 贡献

欢迎贡献代码！请遵循以下步骤：

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 致谢

- [golang-profiling](../golang-profiling) - 核心分析工具
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes Go 客户端
- [cobra](https://github.com/spf13/cobra) - CLI 框架