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

// Profiler performance analyzer
type Profiler struct {
	k8sConfig *config.KubernetesConfig
	discovery *discovery.Discovery
	jobManager *job.Manager
}

// NewProfiler creates a new performance analyzer
func NewProfiler(k8sConfig *config.KubernetesConfig) (*Profiler, error) {
	// Create discovery service
	discoveryService, err := discovery.NewDiscovery(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery service: %w", err)
	}

	// Create Job manager
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

// Profile executes performance analysis
func (p *Profiler) Profile(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions) (*types.ProfileResult, error) {
	// 1. Discover target container
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
		if err := p.cleanup(ctx, result.JobName, cfg.Namespace); err != nil {
			// 记录清理错误但不影响主流程
			fmt.Printf("Warning: failed to cleanup resources: %v\n", err)
		}
	}

	return result, nil
}

// discoverTarget discovers target container
func (p *Profiler) discoverTarget(ctx context.Context, cfg *types.ProfileConfig) (*types.TargetInfo, error) {
	// Find Pod
	pod, err := p.discovery.FindPod(ctx, cfg.Namespace, cfg.PodName)
	if err != nil {
		return nil, fmt.Errorf("failed to find pod: %w", err)
	}

	// Find container
	container, err := p.discovery.FindContainer(pod, cfg.ContainerName)
	if err != nil {
		return nil, fmt.Errorf("failed to find container: %w", err)
	}

	// Get node information
	nodeInfo, err := p.discovery.GetNodeInfo(ctx, pod.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w", err)
	}

	// Get runtime information
	runtimeInfo, err := p.discovery.GetRuntimeInfo(ctx, pod, container)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime info: %w", err)
	}

	// Ensure using the actual found container name
	actualContainerName := cfg.ContainerName
	if actualContainerName == "" && container != nil {
		// If no container name specified, use the first found container name
		actualContainerName = container.Name
	}

	return &types.TargetInfo{
		Namespace:     cfg.Namespace,
		PodName:       cfg.PodName,
		ContainerName: actualContainerName,
		NodeName:      pod.Spec.NodeName,
		Pod:           pod,
		Container:     container,
		NodeInfo:      nodeInfo,
		RuntimeInfo:   runtimeInfo,
	}, nil
}

// executeProfilingJob executes profiling Job
func (p *Profiler) executeProfilingJob(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) (*types.ProfileResult, error) {
	// Create Job and wait for completion
	result, err := p.jobManager.CreateProfilingJobWithMonitoring(ctx, cfg, opts, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create and execute profiling job: %w", err)
	}

	return result, nil
}

// collectResults collects analysis results (simplified version, from logs)
func (p *Profiler) collectResults(ctx context.Context, cfg *types.ProfileConfig, result *types.ProfileResult) (*types.ProfileResult, error) {
	// Extract actual flame graph content from Job logs
	flameGraphData, err := p.jobManager.ExtractFlameGraphFromLogs(ctx, result.JobName, cfg.Namespace)
	if err != nil {
		// If extraction fails, create an error SVG with red X
		errorSVG := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="500" height="300" viewBox="0 0 500 300">
  <!-- 背景 -->
  <rect width="500" height="300" fill="#f8f9fa" stroke="#dee2e6" stroke-width="2"/>
  
  <!-- Red X mark -->
  <g transform="translate(250,100)">
    <circle cx="0" cy="0" r="50" fill="#dc3545" stroke="#b02a37" stroke-width="3"/>
    <line x1="-25" y1="-25" x2="25" y2="25" stroke="white" stroke-width="6" stroke-linecap="round"/>
    <line x1="25" y1="-25" x2="-25" y2="25" stroke="white" stroke-width="6" stroke-linecap="round"/>
  </g>
  
  <!-- Failure message text -->
  <text x="250" y="200" text-anchor="middle" font-family="Arial, sans-serif" font-size="24" font-weight="bold" fill="#dc3545">
    Flame Graph Generation Failed
  </text>
  <text x="250" y="230" text-anchor="middle" font-family="Arial, sans-serif" font-size="14" fill="#6c757d">
    Failed to extract flamegraph from logs
  </text>
  <text x="250" y="250" text-anchor="middle" font-family="Arial, sans-serif" font-size="12" fill="#6c757d">
    Error: ` + err.Error() + `
  </text>
</svg>`
		flameGraphData = []byte(errorSVG)
	}
	
	if cfg.OutputPath != "" {
		if err := p.saveOutputFile(cfg.OutputPath, flameGraphData); err != nil {
			return nil, fmt.Errorf("failed to save output file: %w", err)
		}
		
		result.OutputPath = cfg.OutputPath
		result.FileSize = int64(len(flameGraphData))
	}

	return result, nil
}

// saveOutputFile saves output file
func (p *Profiler) saveOutputFile(outputPath string, data []byte) error {
	if outputPath == "" {
		return fmt.Errorf("output path is empty")
	}

	// Handle path: if relative path, base on current working directory
	var finalPath string
	if filepath.IsAbs(outputPath) {
		finalPath = outputPath
	} else {
		// 获取当前工作目录
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		finalPath = filepath.Join(cwd, outputPath)
	}

	// 确保输出目录存在
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(finalPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Flamegraph saved to: %s\n", finalPath)
	return nil
}

// cleanup 清理资源
func (p *Profiler) cleanup(ctx context.Context, jobName string, namespace string) error {
	return p.jobManager.DeleteJob(ctx, jobName, namespace)
}

// GetStatus 获取分析状态
func (p *Profiler) GetStatus(ctx context.Context, jobName string, namespace string) (*types.JobStatus, error) {
	return p.jobManager.GetJobStatus(ctx, jobName, namespace)
}

// ListJobs 列出所有分析Job（简化版本）
func (p *Profiler) ListJobs(ctx context.Context, namespace string) ([]*types.JobStatus, error) {
	// 在简化架构中，我们不再维护Job列表
	// 返回空列表
	return []*types.JobStatus{}, nil
}

// Cancel 取消分析
func (p *Profiler) Cancel(ctx context.Context, jobName string, namespace string) error {
	return p.jobManager.DeleteJob(ctx, jobName, namespace)
}