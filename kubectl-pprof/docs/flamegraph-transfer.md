# 火焰图回传机制

本文档详细说明了 `kubectl-pprof` 插件如何将生成的火焰图从 Kubernetes 集群回传到本地的技术实现。

## 概述

`kubectl-pprof` 通过以下步骤实现火焰图的生成和回传：

1. **Job 创建**: 在目标节点创建分析 Job
2. **性能分析**: 使用 `golang-profiling` 工具进行分析
3. **火焰图生成**: 在 Job Pod 中生成 SVG 格式的火焰图
4. **数据回传**: 通过多种方式将火焰图传回本地
5. **本地保存**: 保存到指定的输出路径

## 技术实现

### 1. Job Pod 配置

分析 Job 的 Pod 配置包含以下关键特性：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: kubectl-pprof-xxx
  labels:
    app: kubectl-pprof
spec:
  template:
    spec:
      shareProcessNamespace: true  # 共享 PID 命名空间
      containers:
      - name: profiler
        image: golang-profiling:latest
        command: ["/usr/local/bin/golang-profiling"]
        args:
        - "--pid=1"  # 目标进程 PID
        - "--duration=30s"
        - "--output=/tmp/profile.svg"
        - "--flamegraph"
        volumeMounts:
        - name: proc
          mountPath: /proc
        - name: sys
          mountPath: /sys
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      nodeSelector:
        kubernetes.io/hostname: target-node
```

### 2. 火焰图回传方式

#### 方式一：文件复制（推荐）

通过 Kubernetes API 的 `exec` 接口，使用 `cat` 命令读取 Pod 中的火焰图文件：

```go
func (m *Manager) copyFileFromPod(ctx context.Context, podName, filePath string) ([]byte, error) {
    // 执行 cat 命令读取文件内容
    cmd := []string{"cat", filePath}
    req := m.k8sConfig.Clientset.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(podName).
        Namespace(m.k8sConfig.Namespace).
        SubResource("exec")

    req.VersionedParams(&corev1.PodExecOptions{
        Command: cmd,
        Stdout:  true,
        Stderr:  true,
    }, scheme.ParameterCodec)

    exec, err := remotecommand.NewSPDYExecutor(m.k8sConfig.Config, "POST", req.URL())
    if err != nil {
        return nil, fmt.Errorf("failed to create executor: %w", err)
    }

    var stdout, stderr bytes.Buffer
    err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
        Stdout: &stdout,
        Stderr: &stderr,
    })

    if err != nil {
        return nil, fmt.Errorf("failed to execute command: %w", err)
    }

    return stdout.Bytes(), nil
}
```

#### 方式二：日志输出（备用）

当文件复制失败时，从 Pod 日志中获取 base64 编码的火焰图数据：

```go
func (m *Manager) getOutputFromLogs(ctx context.Context, podName string) ([]byte, error) {
    // 获取 Pod 日志
    req := m.k8sConfig.Clientset.CoreV1().Pods(m.k8sConfig.Namespace).GetLogs(podName, &corev1.PodLogOptions{})
    logs, err := req.Stream(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get pod logs: %w", err)
    }
    defer logs.Close()

    // 读取日志内容
    logData, err := io.ReadAll(logs)
    if err != nil {
        return nil, fmt.Errorf("failed to read logs: %w", err)
    }

    // 查找 base64 编码的文件内容
    logStr := string(logData)
    lines := strings.Split(logStr, "\n")
    for _, line := range lines {
        if strings.HasPrefix(line, "FLAMEGRAPH_DATA: ") {
            base64Data := strings.TrimPrefix(line, "FLAMEGRAPH_DATA: ")
            data, err := base64.StdEncoding.DecodeString(base64Data)
            if err != nil {
                return nil, fmt.Errorf("failed to decode base64 data: %w", err)
            }
            return data, nil
        }
    }

    return nil, fmt.Errorf("no flamegraph data found in logs")
}
```

### 3. 本地文件保存

获取火焰图数据后，保存到本地指定路径：

```go
func (p *Profiler) saveOutputFile(outputPath string, data []byte) error {
    if outputPath == "" {
        return fmt.Errorf("output path is empty")
    }

    // 确保输出目录存在
    dir := filepath.Dir(outputPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    // 写入文件
    if err := os.WriteFile(outputPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write output file: %w", err)
    }

    fmt.Printf("Flamegraph saved to: %s\n", outputPath)
    return nil
}
```

## 使用示例

### 基本用法

```bash
# 生成火焰图并保存到默认路径
kubectl pprof my-namespace my-pod

# 指定输出路径
kubectl pprof -o /tmp/my-flamegraph.svg my-namespace my-pod

# 指定容器和分析时长
kubectl pprof -d 60s my-namespace my-pod my-container
```

### 高级用法

```bash
# 生成内存分析火焰图
kubectl pprof --type memory -o memory-profile.svg my-namespace my-pod

# 使用自定义分析镜像
kubectl pprof -i my-registry/golang-profiling:v1.0 my-namespace my-pod

# 生成多种格式输出
kubectl pprof --format png -o profile.png my-namespace my-pod
kubectl pprof --json -o profile.json my-namespace my-pod
```

## 故障排除

### 常见问题

1. **文件复制失败**
   - 检查 Pod 是否成功完成
   - 确认火焰图文件是否生成在 `/tmp/profile.svg`
   - 查看 Pod 日志获取详细错误信息

2. **权限问题**
   - 确保有足够的 RBAC 权限访问 Pod 和执行命令
   - 检查是否启用了 `privileged` 模式

3. **网络问题**
   - 确认 Kubernetes API 服务器连接正常
   - 检查集群网络策略是否阻止了连接

### 调试命令

```bash
# 查看 Job 状态
kubectl get jobs -l app=kubectl-pprof

# 查看 Pod 日志
kubectl logs -l job-name=kubectl-pprof-xxx

# 手动进入 Pod 检查文件
kubectl exec -it <pod-name> -- ls -la /tmp/
```

## 性能考虑

1. **文件大小**: SVG 火焰图通常在几 KB 到几 MB 之间
2. **传输时间**: 取决于文件大小和网络延迟
3. **资源使用**: 文件复制过程消耗少量 CPU 和内存
4. **并发限制**: 建议避免同时运行过多分析任务

## 安全注意事项

1. **权限控制**: 仅授予必要的 RBAC 权限
2. **数据敏感性**: 火焰图可能包含函数名等敏感信息
3. **网络安全**: 确保 Kubernetes API 通信加密
4. **文件清理**: 及时清理临时文件和 Job 资源