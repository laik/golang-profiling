package config

import (
	"fmt"

	"github.com/withlin/kubectl-pprof/internal/types"
)

// LanguageManager manages language-specific configurations
type LanguageManager struct {
	configs map[types.Language]*types.LanguageConfig
}

// NewLanguageManager creates a new language manager with default configurations
func NewLanguageManager() *LanguageManager {
	lm := &LanguageManager{
		configs: make(map[types.Language]*types.LanguageConfig),
	}
	lm.initializeDefaultConfigs()
	return lm
}

// GetConfig returns the configuration for a specific language
func (lm *LanguageManager) GetConfig(lang types.Language) (*types.LanguageConfig, error) {
	if config, exists := lm.configs[lang]; exists {
		return config, nil
	}
	return nil, fmt.Errorf("unsupported language: %s", lang)
}

// IsSupported checks if a language is supported
func (lm *LanguageManager) IsSupported(lang types.Language) bool {
	_, exists := lm.configs[lang]
	return exists
}

// GetSupportedLanguages returns a list of all supported languages
func (lm *LanguageManager) GetSupportedLanguages() []types.Language {
	languages := make([]types.Language, 0, len(lm.configs))
	for lang := range lm.configs {
		languages = append(languages, lang)
	}
	return languages
}

// ValidateProfileType checks if a profile type is valid for the given language
func (lm *LanguageManager) ValidateProfileType(lang types.Language, profileType string) error {
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

// initializeDefaultConfigs sets up default configurations for supported languages
func (lm *LanguageManager) initializeDefaultConfigs() {
	// Go language configuration
	lm.configs[types.LanguageGo] = &types.LanguageConfig{
		Language:       types.LanguageGo,
		SupportedTypes: []string{"cpu", "memory", "goroutine", "block", "mutex", "heap", "allocs"},
		DefaultType:    "cpu",
		DefaultImage:   "golang-profiling:latest",
		ProfilerCommand: []string{"/usr/local/bin/golang-profiling"},
		OutputFormats:  []string{"svg", "png", "pdf", "json", "raw"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"PROFILER_TYPE": "go",
		},
	}

	// Java language configuration
	lm.configs[types.LanguageJava] = &types.LanguageConfig{
		Language:       types.LanguageJava,
		SupportedTypes: []string{"cpu", "memory", "allocation", "lock", "wall"},
		DefaultType:    "cpu",
		DefaultImage:   "java-profiling:latest",
		ProfilerCommand: []string{"/usr/local/bin/async-profiler"},
		OutputFormats:  []string{"svg", "html", "json", "jfr"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"PROFILER_TYPE": "java",
			"JAVA_TOOL_OPTIONS": "-XX:+UnlockDiagnosticVMOptions -XX:+DebugNonSafepoints",
		},
	}

	// Python language configuration
	lm.configs[types.LanguagePython] = &types.LanguageConfig{
		Language:       types.LanguagePython,
		SupportedTypes: []string{"cpu", "memory"},
		DefaultType:    "cpu",
		DefaultImage:   "python-profiling:latest",
		ProfilerCommand: []string{"/usr/local/bin/py-spy"},
		OutputFormats:  []string{"svg", "json", "speedscope"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"PROFILER_TYPE": "python",
		},
	}

	// Node.js language configuration
	lm.configs[types.LanguageNode] = &types.LanguageConfig{
		Language:       types.LanguageNode,
		SupportedTypes: []string{"cpu", "memory", "heap"},
		DefaultType:    "cpu",
		DefaultImage:   "node-profiling:latest",
		ProfilerCommand: []string{"/usr/local/bin/clinic"},
		OutputFormats:  []string{"svg", "json", "html"},
		RequiredCapabilities: []string{"SYS_PTRACE"},
		EnvironmentVars: map[string]string{
			"PROFILER_TYPE": "node",
		},
	}

	// Rust language configuration
	lm.configs[types.LanguageRust] = &types.LanguageConfig{
		Language:       types.LanguageRust,
		SupportedTypes: []string{"cpu", "memory"},
		DefaultType:    "cpu",
		DefaultImage:   "rust-profiling:latest",
		ProfilerCommand: []string{"/usr/local/bin/perf"},
		OutputFormats:  []string{"svg", "json"},
		RequiredCapabilities: []string{"SYS_PTRACE", "SYS_ADMIN"},
		EnvironmentVars: map[string]string{
			"PROFILER_TYPE": "rust",
		},
	}
}