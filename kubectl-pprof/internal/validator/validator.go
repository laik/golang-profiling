package validator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/withlin/kubectl-pprof/internal/errors"
	"github.com/withlin/kubectl-pprof/internal/types"
)

// Validator provides comprehensive validation for profiling configurations
type Validator struct {
	langManager *types.LanguageManager
}

// NewValidator creates a new validator instance
func NewValidator(langManager *types.LanguageManager) *Validator {
	return &Validator{
		langManager: langManager,
	}
}

// ValidateConfig performs comprehensive validation of profiling configuration
func (v *Validator) ValidateConfig(cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	if cfg == nil {
		return errors.NewValidationError(
			"profile configuration is required",
			"Ensure you provide a valid ProfileConfig object",
		)
	}
	if opts == nil {
		return errors.NewValidationError(
			"profile options are required",
			"Ensure you provide a valid ProfileOptions object",
		)
	}

	// Validate required fields
	if err := v.validateRequiredFields(cfg); err != nil {
		return err
	}

	// Validate Kubernetes-specific fields
	if err := v.validateKubernetesFields(cfg); err != nil {
		return err
	}

	// Validate timing parameters
	if err := v.validateTimingParameters(cfg); err != nil {
		return err
	}

	// Validate language and profile type
	if err := v.validateLanguageConfig(cfg); err != nil {
		return err
	}

	// Validate output configuration
	if err := v.validateOutputConfig(cfg, opts); err != nil {
		return err
	}

	// Validate resource limits
	if err := v.validateResourceLimits(cfg); err != nil {
		return err
	}

	return nil
}

// validateRequiredFields validates that all required fields are present
func (v *Validator) validateRequiredFields(cfg *types.ProfileConfig) error {
	if strings.TrimSpace(cfg.Namespace) == "" {
		return errors.NewValidationError(
			"target namespace is required",
			"Use --target-namespace or -n to specify the namespace",
			"Example: kubectl-pprof -n kube-system -p my-pod",
		)
	}

	if strings.TrimSpace(cfg.PodName) == "" {
		return errors.NewValidationError(
			"target pod name is required",
			"Use --target-pod or -p to specify the pod name",
			"Example: kubectl-pprof -n kube-system -p my-pod",
		)
	}

	if strings.TrimSpace(cfg.ProfileType) == "" {
		return errors.NewValidationError(
			"profile type is required",
			"Use --profile-type or -t to specify the profile type",
			"Example: --profile-type cpu (default: cpu)",
		)
	}

	if strings.TrimSpace(cfg.OutputPath) == "" {
		return errors.NewValidationError(
			"output path is required",
			"Use --output or -o to specify the output file path",
			"Example: --output flamegraph.svg",
		)
	}

	if strings.TrimSpace(cfg.Image) == "" {
		return errors.NewValidationError(
			"profiling image is required",
			"Use --image to specify the profiling tool image",
			"Example: --image golang-profiling:latest",
		)
	}

	return nil
}

// validateKubernetesFields validates Kubernetes-specific field formats
func (v *Validator) validateKubernetesFields(cfg *types.ProfileConfig) error {
	// Validate namespace format (RFC 1123 DNS label)
	if !isValidKubernetesName(cfg.Namespace) {
		return errors.NewValidationError(
			fmt.Sprintf("invalid namespace format: %s", cfg.Namespace),
			"Namespace must be a valid DNS label (lowercase alphanumeric and hyphens)",
			"Example: kube-system, default, my-namespace",
		)
	}

	// Validate pod name format
	if !isValidKubernetesName(cfg.PodName) {
		return errors.NewValidationError(
			fmt.Sprintf("invalid pod name format: %s", cfg.PodName),
			"Pod name must be a valid DNS label (lowercase alphanumeric and hyphens)",
			"Example: my-app-12345, web-server-abc",
		)
	}

	// Validate container name if specified
	if cfg.ContainerName != "" && !isValidKubernetesName(cfg.ContainerName) {
		return errors.NewValidationError(
			fmt.Sprintf("invalid container name format: %s", cfg.ContainerName),
			"Container name must be a valid DNS label (lowercase alphanumeric and hyphens)",
			"Example: app, web-server, api-container",
		)
	}

	// Validate job name if specified
	if cfg.JobName != "" && !isValidKubernetesName(cfg.JobName) {
		return errors.NewValidationError(
			fmt.Sprintf("invalid job name format: %s", cfg.JobName),
			"Job name must be a valid DNS label (lowercase alphanumeric and hyphens)",
			"Example: kubectl-pprof, profiling-job",
		)
	}

	return nil
}

// validateTimingParameters validates duration and timeout settings
func (v *Validator) validateTimingParameters(cfg *types.ProfileConfig) error {
	const (
		minDuration = 1 * time.Second
		maxDuration = 10 * time.Minute
		minTimeout  = 30 * time.Second
		maxTimeout  = 30 * time.Minute
	)

	if cfg.Duration <= 0 {
		return errors.NewValidationError(
			"profiling duration must be positive",
			"Use --duration to specify a positive duration",
			"Example: --duration 30s, --duration 2m",
		)
	}

	if cfg.Duration < minDuration {
		return errors.NewValidationError(
			fmt.Sprintf("profiling duration too short (minimum %v, got %v)", minDuration, cfg.Duration),
			fmt.Sprintf("Use a duration of at least %v", minDuration),
			"Example: --duration 30s",
		)
	}

	if cfg.Duration > maxDuration {
		return errors.NewValidationError(
			fmt.Sprintf("profiling duration too long (maximum %v, got %v)", maxDuration, cfg.Duration),
			fmt.Sprintf("Use a duration of at most %v", maxDuration),
			"Example: --duration 5m",
		)
	}

	if cfg.Timeout <= 0 {
		return errors.NewValidationError(
			"timeout must be positive",
			"Use --timeout to specify a positive timeout",
			"Example: --timeout 5m",
		)
	}

	if cfg.Timeout < minTimeout {
		return errors.NewValidationError(
			fmt.Sprintf("timeout too short (minimum %v, got %v)", minTimeout, cfg.Timeout),
			fmt.Sprintf("Use a timeout of at least %v", minTimeout),
			"Example: --timeout 2m",
		)
	}

	if cfg.Timeout > maxTimeout {
		return errors.NewValidationError(
			fmt.Sprintf("timeout too long (maximum %v, got %v)", maxTimeout, cfg.Timeout),
			fmt.Sprintf("Use a timeout of at most %v", maxTimeout),
			"Example: --timeout 15m",
		)
	}

	if cfg.Timeout <= cfg.Duration {
		return errors.NewValidationError(
			fmt.Sprintf("timeout (%v) must be greater than duration (%v)", cfg.Timeout, cfg.Duration),
			"Ensure timeout is longer than profiling duration",
			"Example: --duration 30s --timeout 2m",
		)
	}

	return nil
}

// validateLanguageConfig validates language and profile type compatibility
func (v *Validator) validateLanguageConfig(cfg *types.ProfileConfig) error {
	if v.langManager == nil {
		return errors.NewConfigurationError(
			"language manager not initialized",
			nil,
			"This is an internal error, please report it",
		)
	}

	// Parse language
	lang, err := types.ParseLanguage(cfg.Language)
	if err != nil {
		supportedLangs := v.langManager.GetSupportedLanguages()
		supportedLangStrs := make([]string, len(supportedLangs))
		for i, l := range supportedLangs {
			supportedLangStrs[i] = string(l)
		}
		return errors.NewValidationError(
			fmt.Sprintf("unsupported language: %s", cfg.Language),
			fmt.Sprintf("Use one of the supported languages: %s", strings.Join(supportedLangStrs, ", ")),
			"Example: --lang go, --lang java, --lang python",
		)
	}

	// Validate profile type for the language
	if err := v.langManager.ValidateProfileType(lang, cfg.ProfileType); err != nil {
		langConfig, _ := v.langManager.GetConfig(lang)
		supportedTypes := "unknown"
		if langConfig != nil {
			supportedTypes = strings.Join(langConfig.SupportedTypes, ", ")
		}
		return errors.NewValidationError(
			fmt.Sprintf("unsupported profile type '%s' for language '%s'", cfg.ProfileType, cfg.Language),
			fmt.Sprintf("Use one of the supported profile types for %s: %s", cfg.Language, supportedTypes),
			fmt.Sprintf("Example: --lang %s --profile-type %s", cfg.Language, strings.Split(supportedTypes, ", ")[0]),
		)
	}

	return nil
}

// validateOutputConfig validates output format and path settings
func (v *Validator) validateOutputConfig(cfg *types.ProfileConfig, opts *types.ProfileOptions) error {
	validFormats := map[string]bool{
		"svg": true, "png": true, "pdf": true,
		"json": true, "html": true, "raw": true,
		"flamegraph": true, "collapsed": true,
	}

	if !validFormats[opts.OutputFormat] {
		validFormatsList := make([]string, 0, len(validFormats))
		for format := range validFormats {
			validFormatsList = append(validFormatsList, format)
		}
		return errors.NewValidationError(
			fmt.Sprintf("invalid output format: %s", opts.OutputFormat),
			fmt.Sprintf("Use one of the supported formats: %s", strings.Join(validFormatsList, ", ")),
			"Example: --output-format svg, --output-format json",
		)
	}

	// Validate output path
	if !isValidFilePath(cfg.OutputPath) {
		return errors.NewValidationError(
			fmt.Sprintf("invalid output path: %s", cfg.OutputPath),
			"Use a valid file path for output",
			"Example: --output ./flamegraph.svg, --output /tmp/profile.json",
		)
	}

	// Validate sample rate
	if opts.SampleRate < 0 {
		return errors.NewValidationError(
			"sample rate cannot be negative",
			"Use a non-negative sample rate (0 for default)",
			"Example: --sample-rate 100, --sample-rate 0",
		)
	}
	if opts.SampleRate > 10000 {
		return errors.NewValidationError(
			"sample rate too high (maximum 10000)",
			"Use a sample rate between 0 and 10000",
			"Example: --sample-rate 1000",
		)
	}

	// Validate stack depth
	if opts.StackDepth < 0 {
		return errors.NewValidationError(
			"stack depth cannot be negative",
			"Use a non-negative stack depth (0 for unlimited)",
			"Example: --stack-depth 50, --stack-depth 0",
		)
	}
	if opts.StackDepth > 1000 {
		return errors.NewValidationError(
			"stack depth too high (maximum 1000)",
			"Use a stack depth between 0 and 1000",
			"Example: --stack-depth 100",
		)
	}

	return nil
}

// validateResourceLimits validates CPU and memory resource limits
func (v *Validator) validateResourceLimits(cfg *types.ProfileConfig) error {
	if cfg.ResourceLimits == nil {
		return nil // Resource limits are optional
	}

	// Validate CPU limit
	if cfg.ResourceLimits.CPU != "" {
		if err := validateCPULimit(cfg.ResourceLimits.CPU); err != nil {
			return errors.NewValidationError(
				fmt.Sprintf("invalid CPU limit: %s", cfg.ResourceLimits.CPU),
				"Use a valid CPU limit format",
				"Example: --cpu-limit 500m, --cpu-limit 1, --cpu-limit 2.5",
			)
		}
	}

	// Validate memory limit
	if cfg.ResourceLimits.Memory != "" {
		if err := validateMemoryLimit(cfg.ResourceLimits.Memory); err != nil {
			return errors.NewValidationError(
				fmt.Sprintf("invalid memory limit: %s", cfg.ResourceLimits.Memory),
				"Use a valid memory limit format",
				"Example: --memory-limit 512Mi, --memory-limit 1Gi, --memory-limit 2048Mi",
			)
		}
	}

	return nil
}

// Helper functions

// isValidKubernetesName validates Kubernetes resource names (RFC 1123 DNS label)
func isValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	// RFC 1123 DNS label: lowercase alphanumeric and hyphens, start and end with alphanumeric
	pattern := `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

// isValidFilePath validates file paths
func isValidFilePath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	// Check for invalid characters and patterns
	if strings.Contains(path, "\x00") {
		return false
	}
	// Ensure it's a valid file path (not just a directory)
	ext := filepath.Ext(path)
	return ext != "" || !strings.HasSuffix(path, "/")
}

// validateCPULimit validates Kubernetes CPU limit format
func validateCPULimit(cpu string) error {
	if cpu == "0" || cpu == "0m" {
		return fmt.Errorf("CPU limit cannot be zero")
	}
	
	// Handle millicpu format (e.g., "500m")
	if strings.HasSuffix(cpu, "m") {
		milliStr := strings.TrimSuffix(cpu, "m")
		milli, err := strconv.Atoi(milliStr)
		if err != nil || milli <= 0 {
			return fmt.Errorf("invalid millicpu format")
		}
		return nil
	}
	
	// Handle decimal format (e.g., "1.5")
	value, err := strconv.ParseFloat(cpu, 64)
	if err != nil || value <= 0 {
		return fmt.Errorf("invalid CPU value")
	}
	
	return nil
}

// validateMemoryLimit validates Kubernetes memory limit format
func validateMemoryLimit(memory string) error {
	if memory == "0" || memory == "0Mi" || memory == "0Gi" || memory == "0Ki" {
		return fmt.Errorf("memory limit cannot be zero")
	}
	
	// Common memory suffixes
	validSuffixes := []string{"Ki", "Mi", "Gi", "Ti", "K", "M", "G", "T"}
	
	for _, suffix := range validSuffixes {
		if strings.HasSuffix(memory, suffix) {
			valueStr := strings.TrimSuffix(memory, suffix)
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil || value <= 0 {
				return fmt.Errorf("invalid memory value")
			}
			return nil
		}
	}
	
	// Handle plain number (bytes)
	value, err := strconv.ParseInt(memory, 10, 64)
	if err != nil || value <= 0 {
		return fmt.Errorf("invalid memory format")
	}
	
	return nil
}