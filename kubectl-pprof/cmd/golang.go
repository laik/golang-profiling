package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withlin/kubectl-pprof/internal/types"
)

// newGolangCmd 创建 golang 子命令
func newGolangCmd(cfg *types.ProfileConfig, opts *types.ProfileOptions) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "golang [flags]",
		Short: "Profile Go applications",
		Long:  `Profile Go applications using pprof`,
		SilenceUsage: true, // 禁止在错误时显示用法信息
		RunE: func(cmd *cobra.Command, args []string) error {
			// 设置语言为 Go
			cfg.Language = "go"
			return runProfile(cmd.Context(), cfg, opts)
		},
	}

	// Go语言基本参数
	var (
		pid             int
		duration        int
		output          string
		frequency       int
		image           string
		imagePullPolicy string
	)

	cmd.Flags().IntVar(&pid, "pid", 0, "Process ID to profile (0 = auto-detect by crictl)")
	cmd.Flags().IntVar(&duration, "duration", 5, "Duration of profiling in seconds")
	cmd.Flags().StringVar(&output, "output", "/tmp/profile.svg", "Output file path")
	cmd.Flags().IntVar(&frequency, "frequency", 99, "Sampling frequency for CPU profiling")
	cmd.Flags().StringVar(&image, "image", "golang-profiling:latest", "Profiling tool image")
	cmd.Flags().StringVar(&imagePullPolicy, "image-pull-policy", "IfNotPresent", "Image pull policy (Always, IfNotPresent, Never)")

	// Note: Job configuration, resource limits, and UI options are inherited from parent command

	// Note: Required flags are handled by parent command

	// Set up pre-run to configure Go options
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// 设置默认配置
		cfg.Language = "go"
		cfg.ProfileType = "cpu"
		
		// 设置Go特定配置
		// 只有当用户明确指定了pid且不为0时才设置PID
		if pid > 0 {
			cfg.PID = fmt.Sprintf("%d", pid)
		}
		// 如果pid为0或未指定，保持cfg.PID为空，让crictl自动探测
		cfg.Duration = time.Duration(duration) * time.Second
		
		// 只有当用户明确指定了output参数时才覆盖，否则使用父命令的OutputPath
		if cmd.Flags().Changed("output") {
			cfg.OutputPath = output
		}
		
		// 设置镜像配置
		if cmd.Flags().Changed("image") {
			cfg.Image = image
		}
		if cmd.Flags().Changed("image-pull-policy") {
			cfg.ImagePullPolicy = imagePullPolicy
		}
		
		// Configure Go-specific options
		cfg.GoOptions = &types.GoProfilingOptions{
			Frequency: frequency,
		}

		// Validate configuration
		if err := validateGoConfig(cfg, opts); err != nil {
			return fmt.Errorf("Go configuration validation failed: %w", err)
		}

		return nil
	}

	return cmd
}

// validateGoConfig 验证 Go 特定的配置
func validateGoConfig(cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	// 验证命名空间
	if cfg.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	// 验证 Pod 名称
	if cfg.PodName == "" {
		return fmt.Errorf("pod name is required")
	}

	// 只支持CPU分析
	if cfg.ProfileType != "cpu" {
		cfg.ProfileType = "cpu"
	}

	// 验证持续时间
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if cfg.Duration > 10*time.Minute {
		return fmt.Errorf("duration cannot exceed 10 minutes for safety")
	}

	// 验证采样频率
	if cfg.GoOptions != nil && cfg.GoOptions.Frequency > 0 {
		if cfg.GoOptions.Frequency < 1 || cfg.GoOptions.Frequency > 10000 {
			return fmt.Errorf("frequency must be between 1 and 10000 Hz")
		}
	}

	// 验证图像尺寸
	if cfg.GoOptions != nil {
		if cfg.GoOptions.Width > 0 && (cfg.GoOptions.Width < 400 || cfg.GoOptions.Width > 5000) {
			return fmt.Errorf("width must be between 400 and 5000 pixels")
		}
		if cfg.GoOptions.Height > 0 && (cfg.GoOptions.Height < 10 || cfg.GoOptions.Height > 100) {
			return fmt.Errorf("height must be between 10 and 100 pixels")
		}
		if cfg.GoOptions.FontSize > 0 && (cfg.GoOptions.FontSize < 6 || cfg.GoOptions.FontSize > 24) {
			return fmt.Errorf("font size must be between 6 and 24")
		}
	}

	// 验证颜色方案
	if cfg.GoOptions != nil && cfg.GoOptions.Colors != "" {
		validColors := []string{"hot", "mem", "io", "wakeup", "chain", "java", "js", "perl", "red", "green", "blue", "aqua", "yellow", "purple", "orange", "kernel_user"}
		valid := false
		for _, c := range validColors {
			if cfg.GoOptions.Colors == c {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid color scheme '%s', must be one of: %s", cfg.GoOptions.Colors, strings.Join(validColors, ", "))
		}
	}

	// 验证镜像拉取策略
	if cfg.ImagePullPolicy != "" {
		validPolicies := []string{"Always", "IfNotPresent", "Never"}
		valid := false
		for _, p := range validPolicies {
			if cfg.ImagePullPolicy == p {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid image pull policy '%s', must be one of: %s", cfg.ImagePullPolicy, strings.Join(validPolicies, ", "))
		}
	}

	return nil
}