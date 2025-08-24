// Package types provides language-specific configuration and management for profiling tools.
package types

import (
	"fmt"
	"strings"
)

// LanguageManager manages language-specific configurations for profiling
type LanguageManager struct {
	configs map[Language]*LanguageConfig
}

// NewLanguageManager creates a new language manager with default configurations
func NewLanguageManager() *LanguageManager {
	lm := &LanguageManager{
		configs: make(map[Language]*LanguageConfig),
	}
	lm.initializeDefaultConfigs()
	return lm
}

// GetConfig returns the configuration for a specific language
func (lm *LanguageManager) GetConfig(lang Language) (*LanguageConfig, error) {
	config, exists := lm.configs[lang]
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	return config, nil
}

// GetSupportedLanguages returns a list of all supported languages
func (lm *LanguageManager) GetSupportedLanguages() []Language {
	languages := make([]Language, 0, len(lm.configs))
	for lang := range lm.configs {
		languages = append(languages, lang)
	}
	return languages
}

// ValidateProfileType checks if a profile type is supported for the given language
func (lm *LanguageManager) ValidateProfileType(lang Language, profileType string) error {
	config, err := lm.GetConfig(lang)
	if err != nil {
		return err
	}
	
	for _, supportedType := range config.SupportedTypes {
		if supportedType == profileType {
			return nil
		}
	}
	
	return fmt.Errorf("profile type '%s' is not supported for language '%s'. Supported types: %v",
		profileType, lang, config.SupportedTypes)
}

// ParseLanguage converts a string to Language type
func ParseLanguage(langStr string) (Language, error) {
	langStr = strings.ToLower(strings.TrimSpace(langStr))
	
	switch langStr {
	case "go", "golang":
		return LanguageGo, nil
	case "java", "jvm":
		return LanguageJava, nil
	case "python", "py":
		return LanguagePython, nil
	case "node", "nodejs", "javascript", "js":
		return LanguageNode, nil
	case "rust", "rs":
		return LanguageRust, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", langStr)
	}
}

// initializeDefaultConfigs sets up default configurations for all supported languages
func (lm *LanguageManager) initializeDefaultConfigs() {
	// Go language configuration
	lm.configs[LanguageGo] = &LanguageConfig{
		Language:             LanguageGo,
		SupportedTypes:       []string{"cpu", "memory", "goroutine", "block", "mutex", "heap", "allocs"},
		DefaultType:          "cpu",
		DefaultImage:         "golang-profiling:latest",
		ProfilerCommand:      []string{"/usr/local/bin/golang-profiling"},
		OutputFormats:        []string{"svg", "png", "pdf", "json", "html", "raw"},
		RequiredCapabilities: []string{"SYS_PTRACE", "SYS_ADMIN"},
		EnvironmentVars: map[string]string{
			"GOLANG_PROFILING_MODE": "kubernetes",
			"PROFILING_LANGUAGE":    "go",
		},
	}

	// Java language configuration
	lm.configs[LanguageJava] = &LanguageConfig{
		Language:             LanguageJava,
		SupportedTypes:       []string{"cpu", "memory", "allocation", "lock", "wall"},
		DefaultType:          "cpu",
		DefaultImage:         "async-profiler:latest",
		ProfilerCommand:      []string{"/opt/async-profiler/profiler.sh"},
		OutputFormats:        []string{"svg", "html", "jfr", "collapsed"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"JAVA_TOOL_OPTIONS": "-XX:+UnlockDiagnosticVMOptions -XX:+DebugNonSafepoints",
			"PROFILING_LANGUAGE": "java",
		},
	}

	// Python language configuration
	lm.configs[LanguagePython] = &LanguageConfig{
		Language:             LanguagePython,
		SupportedTypes:       []string{"cpu", "memory", "wall"},
		DefaultType:          "cpu",
		DefaultImage:         "py-spy:latest",
		ProfilerCommand:      []string{"/usr/local/bin/py-spy"},
		OutputFormats:        []string{"svg", "flamegraph", "speedscope", "raw"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"PROFILING_LANGUAGE": "python",
		},
	}

	// Node.js language configuration
	lm.configs[LanguageNode] = &LanguageConfig{
		Language:             LanguageNode,
		SupportedTypes:       []string{"cpu", "memory", "heap"},
		DefaultType:          "cpu",
		DefaultImage:         "node-profiler:latest",
		ProfilerCommand:      []string{"/usr/local/bin/node-profiler"},
		OutputFormats:        []string{"svg", "json", "cpuprofile", "heapprofile"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"NODE_OPTIONS":       "--inspect",
			"PROFILING_LANGUAGE": "node",
		},
	}

	// Rust language configuration
	lm.configs[LanguageRust] = &LanguageConfig{
		Language:             LanguageRust,
		SupportedTypes:       []string{"cpu", "memory"},
		DefaultType:          "cpu",
		DefaultImage:         "rust-profiler:latest",
		ProfilerCommand:      []string{"/usr/local/bin/perf"},
		OutputFormats:        []string{"svg", "flamegraph", "perf"},
		RequiredCapabilities: []string{"SYS_PTRACE", "SYS_ADMIN"},
		EnvironmentVars: map[string]string{
			"PROFILING_LANGUAGE": "rust",
		},
	}
}

// GetProfilerArgs generates profiler-specific arguments based on language and configuration
func (lm *LanguageManager) GetProfilerArgs(lang Language, cfg *ProfileConfig, opts *ProfileOptions) ([]string, error) {
	config, err := lm.GetConfig(lang)
	if err != nil {
		return nil, err
	}

	switch lang {
	case LanguageGo:
		return lm.getGoProfilerArgs(cfg, opts), nil
	case LanguageJava:
		return lm.getJavaProfilerArgs(cfg, opts), nil
	case LanguagePython:
		return lm.getPythonProfilerArgs(cfg, opts), nil
	case LanguageNode:
		return lm.getNodeProfilerArgs(cfg, opts), nil
	case LanguageRust:
		return lm.getRustProfilerArgs(cfg, opts), nil
	default:
		return config.ProfilerCommand, nil
	}
}

// Language-specific argument generators
func (lm *LanguageManager) getGoProfilerArgs(cfg *ProfileConfig, opts *ProfileOptions) []string {
	args := []string{
		"--target-pid", "1", // Will be replaced with actual PID
		"--profile-type", cfg.ProfileType,
		"--duration", cfg.Duration.String(),
		"--output", "/tmp/profile.out",
	}
	
	if opts.SampleRate > 0 {
		args = append(args, "--sample-rate", fmt.Sprintf("%d", opts.SampleRate))
	}
	
	if opts.StackDepth > 0 {
		args = append(args, "--stack-depth", fmt.Sprintf("%d", opts.StackDepth))
	}
	
	return args
}

func (lm *LanguageManager) getJavaProfilerArgs(cfg *ProfileConfig, opts *ProfileOptions) []string {
	args := []string{
		"-e", cfg.ProfileType,
		"-d", cfg.Duration.String(),
		"-f", "/tmp/profile.svg",
		"1", // Will be replaced with actual PID
	}
	
	if opts.SampleRate > 0 {
		args = append(args, "-i", fmt.Sprintf("%dms", 1000/opts.SampleRate))
	}
	
	return args
}

func (lm *LanguageManager) getPythonProfilerArgs(cfg *ProfileConfig, opts *ProfileOptions) []string {
	args := []string{
		"record",
		"-o", "/tmp/profile.svg",
		"-d", cfg.Duration.String(),
		"-p", "1", // Will be replaced with actual PID
	}
	
	if cfg.ProfileType == "memory" {
		args = append(args, "--gil")
	}
	
	if opts.SampleRate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", opts.SampleRate))
	}
	
	return args
}

func (lm *LanguageManager) getNodeProfilerArgs(cfg *ProfileConfig, opts *ProfileOptions) []string {
	args := []string{
		"--profile-type", cfg.ProfileType,
		"--duration", cfg.Duration.String(),
		"--output", "/tmp/profile.cpuprofile",
		"--pid", "1", // Will be replaced with actual PID
	}
	
	return args
}

func (lm *LanguageManager) getRustProfilerArgs(cfg *ProfileConfig, opts *ProfileOptions) []string {
	args := []string{
		"record",
		"-F", "99",
		"-p", "1", // Will be replaced with actual PID
		"--", "sleep", cfg.Duration.String(),
	}
	
	if opts.SampleRate > 0 {
		args[1] = "-F"
		args[2] = fmt.Sprintf("%d", opts.SampleRate)
	}
	
	return args
}