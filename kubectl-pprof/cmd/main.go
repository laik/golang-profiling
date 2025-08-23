package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/withlin/kubectl-pprof/internal/types"
	"github.com/withlin/kubectl-pprof/pkg/config"
	"github.com/withlin/kubectl-pprof/pkg/profiler"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfg types.ProfileConfig
	var opts types.ProfileOptions

	cmd := &cobra.Command{
		Use:   "kubectl-pprof [flags] <namespace> <pod> [container]",
		Short: "Profile Go applications running in Kubernetes pods",
		Long: `kubectl-pprof is a kubectl plugin for profiling Go applications running in Kubernetes pods.

It creates a Job on the target node to perform profiling using shared PID namespace,
then generates flame graphs and other performance analysis outputs.

Examples:
  # Profile the main container in a pod
  kubectl pprof my-namespace my-pod

  # Profile a specific container
  kubectl pprof my-namespace my-pod my-container

  # Profile with custom duration and output
  kubectl pprof -d 60s -o /tmp/profile.svg my-namespace my-pod

  # Use custom profiling image
  kubectl pprof -i my-registry/golang-profiling:v1.0 my-namespace my-pod
`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfile(cmd.Context(), &cfg, &opts, args)
		},
	}

	// 基础参数
	cmd.Flags().DurationVarP(&cfg.Duration, "duration", "d", 30*time.Second, "Profiling duration")
	cmd.Flags().StringVarP(&cfg.OutputPath, "output", "o", "flamegraph.svg", "Output file path")
	cmd.Flags().StringVarP(&cfg.Image, "image", "i", "golang-profiling:latest", "Profiling tool image")
	cmd.Flags().StringVarP(&cfg.NodeName, "node", "n", "", "Force scheduling on specific node")
	cmd.Flags().StringVar(&cfg.JobName, "job-name", "kubectl-pprof", "Job name prefix")
	cmd.Flags().BoolVar(&cfg.Cleanup, "cleanup", true, "Cleanup Job resources after completion")
	cmd.Flags().DurationVar(&cfg.Timeout, "timeout", 5*time.Minute, "Job timeout")
	cmd.Flags().BoolVar(&cfg.Privileged, "privileged", true, "Run profiling container in privileged mode")

	// 分析选项
	cmd.Flags().StringVar(&cfg.ProfileType, "type", "cpu", "Profile type (cpu, memory, goroutine, block, mutex)")
	cmd.Flags().BoolVar(&opts.FlameGraph, "flamegraph", true, "Generate flame graph")
	cmd.Flags().BoolVar(&opts.RawData, "raw", false, "Save raw profiling data")
	cmd.Flags().BoolVar(&opts.JSONReport, "json", false, "Generate JSON report")
	cmd.Flags().StringVar(&opts.OutputFormat, "format", "svg", "Output format (svg, png, pdf, json)")
	cmd.Flags().IntVar(&opts.SampleRate, "sample-rate", 0, "Sampling rate (0 for default)")
	cmd.Flags().IntVar(&opts.StackDepth, "stack-depth", 0, "Maximum stack depth (0 for unlimited)")
	cmd.Flags().StringVar(&opts.FilterPattern, "filter", "", "Filter pattern for function names")
	cmd.Flags().StringVar(&opts.IgnorePattern, "ignore", "", "Ignore pattern for function names")

	// 资源限制
	var cpuLimit, memoryLimit string
	cmd.Flags().StringVar(&cpuLimit, "cpu-limit", "1", "CPU limit for profiling job")
	cmd.Flags().StringVar(&memoryLimit, "memory-limit", "512Mi", "Memory limit for profiling job")

	// 高级选项
	cmd.Flags().StringSliceVar(&cfg.ExtraArgs, "extra-args", nil, "Extra arguments for profiling tool")
	cmd.Flags().StringToStringVar(&cfg.EnvVars, "env", nil, "Environment variables for profiling job")

	// 版本信息
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kubectl-pprof version %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built: %s\n", date)
		},
	})

	// 预处理函数
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// 设置资源限制
		if cpuLimit != "" || memoryLimit != "" {
			cfg.ResourceLimits = &types.ResourceLimits{
				CPU:    cpuLimit,
				Memory: memoryLimit,
			}
		}

		// 验证配置
		return validateConfig(&cfg, &opts)
	}

	return cmd
}

func runProfile(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions, args []string) error {
	// 解析参数
	cfg.Namespace = args[0]
	cfg.PodName = args[1]
	if len(args) > 2 {
		cfg.ContainerName = args[2]
	}

	// 加载Kubernetes配置
	k8sConfig, err := config.LoadKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubernetes config: %w", err)
	}

	// 创建分析器
	profilerClient, err := profiler.NewProfiler(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create profiler: %w", err)
	}

	// 执行分析
	fmt.Printf("Starting profiling of %s/%s", cfg.Namespace, cfg.PodName)
	if cfg.ContainerName != "" {
		fmt.Printf("/%s", cfg.ContainerName)
	}
	fmt.Printf(" for %v...\n", cfg.Duration)

	result, err := profilerClient.Profile(ctx, cfg, opts)
	if err != nil {
		return fmt.Errorf("profiling failed: %w", err)
	}

	// 输出结果
	fmt.Printf("\nProfiling completed successfully!\n")
	fmt.Printf("Output file: %s\n", result.OutputPath)
	fmt.Printf("File size: %d bytes\n", result.FileSize)
	fmt.Printf("Duration: %v\n", result.Duration)
	if result.Samples > 0 {
		fmt.Printf("Samples collected: %d\n", result.Samples)
	}

	return nil
}

func validateConfig(cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	// 验证分析类型
	validTypes := map[string]bool{
		"cpu":       true,
		"memory":    true,
		"goroutine": true,
		"block":     true,
		"mutex":     true,
	}
	if !validTypes[cfg.ProfileType] {
		return fmt.Errorf("invalid profile type: %s", cfg.ProfileType)
	}

	// 验证输出格式
	validFormats := map[string]bool{
		"svg":  true,
		"png":  true,
		"pdf":  true,
		"json": true,
	}
	if !validFormats[opts.OutputFormat] {
		return fmt.Errorf("invalid output format: %s", opts.OutputFormat)
	}

	// 验证持续时间
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if cfg.Duration > 10*time.Minute {
		return fmt.Errorf("duration too long (max 10 minutes)")
	}

	// 验证超时时间
	if cfg.Timeout <= cfg.Duration {
		return fmt.Errorf("timeout must be greater than duration")
	}

	return nil
}