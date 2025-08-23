package profiler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/withlin/kubectl-pprof/internal/types"
	"github.com/withlin/kubectl-pprof/pkg/config"
	"github.com/withlin/kubectl-pprof/pkg/discovery"
	"github.com/withlin/kubectl-pprof/pkg/job"
)

// Profiler 性能分析器
type Profiler struct {
	k8sConfig *config.KubernetesConfig
	discovery *discovery.Discovery
	jobManager *job.Manager
}

// NewProfiler 创建新的性能分析器
func NewProfiler(k8sConfig *config.KubernetesConfig) (*Profiler, error) {
	// 创建发现服务
	discoveryService, err := discovery.NewDiscovery(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery service: %w", err)
	}

	// 创建Job管理器
	jobManager, err := job.NewManager(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	return &Profiler{
		k8sConfig: k8sConfig,
		discovery: discoveryService,
		jobManager: jobManager,
	}, nil
}

// Profile 执行性能分析
func (p *Profiler) Profile(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions) (*types.ProfileResult, error) {
	// 1. 发现目标容器
	targetInfo, err := p.discoverTarget(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to discover target: %w", err)
	}

	// 2. 创建并执行分析Job
	jobResult, err := p.executeProfilingJob(ctx, cfg, opts, targetInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to execute profiling job: %w", err)
	}

	// 3. 收集结果
	result, err := p.collectResults(ctx, cfg, jobResult)
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}

	// 4. 清理资源
	if cfg.Cleanup {
		if err := p.cleanup(ctx, jobResult.JobName); err != nil {
			// 记录清理错误但不影响主流程
			fmt.Printf("Warning: failed to cleanup resources: %v\n", err)
		}
	}

	return result, nil
}

// discoverTarget 发现目标容器
func (p *Profiler) discoverTarget(ctx context.Context, cfg *types.ProfileConfig) (*types.TargetInfo, error) {
	// 查找Pod
	pod, err := p.discovery.FindPod(ctx, cfg.Namespace, cfg.PodName)
	if err != nil {
		return nil, fmt.Errorf("failed to find pod: %w", err)
	}

	// 查找容器
	container, err := p.discovery.FindContainer(pod, cfg.ContainerName)
	if err != nil {
		return nil, fmt.Errorf("failed to find container: %w", err)
	}

	// 获取节点信息
	nodeInfo, err := p.discovery.GetNodeInfo(ctx, pod.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w", err)
	}

	// 获取运行时信息
	runtimeInfo, err := p.discovery.GetRuntimeInfo(ctx, pod, container)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime info: %w", err)
	}

	return &types.TargetInfo{
		Pod:         pod,
		Container:   container,
		NodeInfo:    nodeInfo,
		RuntimeInfo: runtimeInfo,
	}, nil
}

// executeProfilingJob 执行分析Job
func (p *Profiler) executeProfilingJob(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) (*types.JobStatus, error) {
	// 创建Job
	jobName, err := p.jobManager.CreateProfilingJob(ctx, cfg, opts, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create profiling job: %w", err)
	}

	// 等待Job完成
	jobStatus, err := p.jobManager.WaitForCompletion(ctx, jobName, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("job execution failed: %w", err)
	}

	return jobStatus, nil
}

// collectResults 收集分析结果
func (p *Profiler) collectResults(ctx context.Context, cfg *types.ProfileConfig, jobStatus *types.JobStatus) (*types.ProfileResult, error) {
	// 从Job Pod中获取结果文件
	outputData, err := p.jobManager.GetJobOutput(ctx, jobStatus.JobName)
	if err != nil {
		return nil, fmt.Errorf("failed to get job output: %w", err)
	}

	// 保存到本地文件
	if err := p.saveOutputFile(cfg.OutputPath, outputData); err != nil {
		return nil, fmt.Errorf("failed to save output file: %w", err)
	}

	return &types.ProfileResult{
		OutputPath: cfg.OutputPath,
		FileSize:   int64(len(outputData)),
		Duration:   jobStatus.EndTime.Sub(*jobStatus.StartTime),
		Samples:    0, // TODO: 从输出中解析样本数
		JobName:    jobStatus.JobName,
		Success:    true,
	}, nil
}

// saveOutputFile 保存输出文件
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

// cleanup 清理资源
func (p *Profiler) cleanup(ctx context.Context, jobName string) error {
	return p.jobManager.DeleteJob(ctx, jobName)
}

// GetStatus 获取分析状态
func (p *Profiler) GetStatus(ctx context.Context, jobName string) (*types.JobStatus, error) {
	return p.jobManager.GetJobStatus(ctx, jobName)
}

// ListJobs 列出所有分析Job
func (p *Profiler) ListJobs(ctx context.Context, namespace string) ([]*types.JobStatus, error) {
	return p.jobManager.ListJobs(ctx, namespace)
}

// Cancel 取消分析
func (p *Profiler) Cancel(ctx context.Context, jobName string) error {
	return p.jobManager.DeleteJob(ctx, jobName)
}