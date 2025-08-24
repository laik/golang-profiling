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

// Build information set by ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// cobra已经通过RunE返回的错误自动输出了错误信息
		// 这里不需要再次输出，避免重复
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfg types.ProfileConfig
	var opts types.ProfileOptions

	cmd := &cobra.Command{
		Use:   "kubectl-pprof [flags]",
		Short: "Profile applications running in Kubernetes pods",
		Long: `kubectl-pprof is a kubectl plugin for profiling Go applications running in Kubernetes pods.

It creates a Job on the target node to perform CPU profiling using shared PID namespace,
then generates flame graphs and other performance analysis outputs.

Examples:
  # Basic Go CPU profiling
  kubectl pprof -n kube-system -p kube-apiserver -c kube-apiserver

  # Profile with custom duration
  kubectl pprof -n default -p my-go-app -d 60s

  # Use golang subcommand for advanced Go profiling with off-CPU analysis
  kubectl pprof golang -n kube-system -p kube-apiserver --off-cpu --frequency 199

  # Advanced Go profiling with custom flame graph settings
  kubectl pprof golang -n production -p api-server --go-title "API Server CPU" --go-width 1600 --go-height 20

  # Export folded stacks for external analysis
  kubectl pprof golang -n monitoring -p prometheus --go-export-folded /output/stacks.folded
`,
		SilenceUsage: true, // 禁止在错误时显示用法信息
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfile(cmd.Context(), &cfg, &opts)
		},
	}

	// Add subcommands
	cmd.AddCommand(newGolangCmd(&cfg, &opts))

	// Target specification (kubectl-prof style with aliases) - 使用PersistentFlags让子命令继承
	cmd.PersistentFlags().StringVarP(&cfg.Namespace, "target-namespace", "n", "", "Target namespace (required)")
	cmd.PersistentFlags().StringVarP(&cfg.PodName, "target-pod", "p", "", "Target pod name (required)")
	cmd.PersistentFlags().StringVarP(&cfg.ContainerName, "container", "c", "", "Target container name")
	cmd.PersistentFlags().StringVar(&cfg.PID, "pid", "", "Specific process ID to profile (default: auto-detect by crictl)")
	cmd.MarkPersistentFlagRequired("target-namespace")
	cmd.MarkPersistentFlagRequired("target-pod")

	// Profiling options (CPU only) - 使用PersistentFlags让子命令继承
	cmd.PersistentFlags().DurationVarP(&cfg.Duration, "duration", "d", 30*time.Second, "Profiling duration")

	// Note: Go-specific options (off-cpu, frequency, etc.) are available in 'golang' subcommand

	// Output options - 使用PersistentFlags让子命令继承
	cmd.PersistentFlags().StringVarP(&cfg.OutputPath, "output", "o", "flamegraph.svg", "Output file path")
	cmd.PersistentFlags().StringVar(&opts.OutputFormat, "output-format", "svg", "Output format (svg, png, pdf, json)")
	cmd.PersistentFlags().BoolVar(&opts.FlameGraph, "flamegraph", true, "Generate flame graph")

	// Job configuration
	cmd.Flags().StringVar(&cfg.Image, "image", "golang-profiling:latest", "Profiling tool image")
	cmd.Flags().StringVar(&cfg.ImagePullPolicy, "image-pull-policy", "IfNotPresent", "Image pull policy (Always, IfNotPresent, Never)")
	cmd.Flags().StringVar(&cfg.NodeName, "node", "", "Force scheduling on specific node")
	cmd.Flags().StringVar(&cfg.JobName, "job-name", "kubectl-pprof", "Job name prefix")
	cmd.Flags().BoolVar(&cfg.Cleanup, "cleanup", true, "Cleanup Job resources after completion")
	cmd.Flags().DurationVar(&cfg.Timeout, "timeout", 5*time.Minute, "Job timeout")
	cmd.Flags().BoolVar(&cfg.Privileged, "privileged", true, "Run profiling container in privileged mode")

	// UI options - 使用PersistentFlags让子命令继承
	cmd.PersistentFlags().BoolVarP(&opts.Quiet, "quiet", "q", false, "Suppress interactive prompts and progress output")
	cmd.PersistentFlags().BoolVar(&opts.PrintLogs, "print-logs", false, "Print profiling job logs to console")

	// Resource limits (simplified with defaults)
	var cpuLimit, memoryLimit string
	cmd.Flags().StringVar(&cpuLimit, "cpu-limit", "1000m", "CPU limit for profiling job")
	cmd.Flags().StringVar(&memoryLimit, "memory-limit", "512Mi", "Memory limit for profiling job")

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

	// Add kubectl-prof style aliases for common flags
	cmd.Flags().StringP("namespace", "", "", "Alias for --target-namespace")
	cmd.Flags().StringP("pod", "", "", "Alias for --target-pod")

	cmd.Flags().StringP("time", "", "", "Alias for --duration")
	cmd.Flags().StringP("out", "", "", "Alias for --output")
	cmd.Flags().StringP("format", "", "", "Alias for --output-format")
	cmd.Flags().BoolP("flame", "", false, "Alias for --flamegraph")
	cmd.Flags().BoolP("clean", "", false, "Alias for --cleanup")
	cmd.Flags().StringP("img", "", "", "Alias for --image")

	// Pre-run validation and setup
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Handle aliases
		if namespace, _ := cmd.Flags().GetString("namespace"); namespace != "" && cfg.Namespace == "" {
			cfg.Namespace = namespace
		}
		if pod, _ := cmd.Flags().GetString("pod"); pod != "" && cfg.PodName == "" {
			cfg.PodName = pod
		}

		if timeStr, _ := cmd.Flags().GetString("time"); timeStr != "" {
			if duration, err := time.ParseDuration(timeStr); err == nil {
				cfg.Duration = duration
			}
		}
		if output, _ := cmd.Flags().GetString("out"); output != "" && cfg.OutputPath == "flamegraph.svg" {
			cfg.OutputPath = output
		}
		if format, _ := cmd.Flags().GetString("format"); format != "" && opts.OutputFormat == "svg" {
			opts.OutputFormat = format
		}
		if flame, _ := cmd.Flags().GetBool("flame"); cmd.Flags().Changed("flame") {
			opts.FlameGraph = flame
		}
		if clean, _ := cmd.Flags().GetBool("clean"); cmd.Flags().Changed("clean") {
			cfg.Cleanup = clean
		}
		if img, _ := cmd.Flags().GetString("img"); img != "" && cfg.Image == "golang-profiling:latest" {
			cfg.Image = img
		}

		// Set resource limits
		if cpuLimit != "" || memoryLimit != "" {
			cfg.ResourceLimits = &types.ResourceLimits{
				CPU:    cpuLimit,
				Memory: memoryLimit,
			}
		}

		// Set default configuration for Go language (CPU profiling only)
		cfg.Language = "go"
		cfg.ProfileType = "cpu"
		if cfg.Image == "golang-profiling:latest" {
			cfg.Image = "golang-profiling:latest"
		}

		// Note: Go-specific options are configured in golang subcommand
		// Main command only handles basic configuration

		// Set default values for removed parameters
		cfg.CrictlPath = "/usr/bin/crictl"
		if cfg.ExtraArgs == nil {
			cfg.ExtraArgs = []string{}
		}
		if cfg.EnvVars == nil {
			cfg.EnvVars = make(map[string]string)
		}

		// Validate configuration
		return validateConfig(&cfg, &opts)
	}

	return cmd
}

func runProfile(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	// Validate required parameters
	if cfg.Namespace == "" {
		return fmt.Errorf("target namespace is required")
	}
	if cfg.PodName == "" {
		return fmt.Errorf("target pod name is required")
	}

	// Simple output - only basic initialization info
	if !opts.Quiet {
		fmt.Println("ℹ️  🔍 Initializing profiling session...")
	}

	// Load Kubernetes config
	if !opts.Quiet {
		fmt.Println(" Loading Kubernetes configuration... ✅")
	}
	k8sConfig, err := config.LoadKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubernetes config: %w", err)
	}

	// Create profiler
	if !opts.Quiet {
		fmt.Println(" Creating profiler client... ✅")
	}
	profilerClient, err := profiler.NewProfiler(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create profiler: %w", err)
	}

	// Start profiling
	if !opts.Quiet {
		fmt.Println("ℹ️  🚀 Starting profiling job...")
	}

	// Run profiling with simple progress indication
	result, err := profilerClient.Profile(ctx, cfg, opts)
	if err != nil {
		return fmt.Errorf("profiling failed: %w", err)
	}

	if !opts.Quiet {
		fmt.Printf("Profiling completed! Output: %s\n", result.OutputPath)
	}

	return nil
}

// validateConfig performs basic validation of profiling configuration
func validateConfig(cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	// Basic validation
	if cfg.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if cfg.PodName == "" {
		return fmt.Errorf("pod name is required")
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	return nil
}
