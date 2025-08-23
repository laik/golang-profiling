package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateJobName 生成Job名称
func GenerateJobName(prefix string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%d", prefix, timestamp)
}

// ValidateNamespace 验证命名空间名称
func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if len(namespace) > 63 {
		return fmt.Errorf("namespace name too long (max 63 characters)")
	}
	// TODO: 添加更多验证规则
	return nil
}

// ValidatePodName 验证Pod名称
func ValidatePodName(podName string) error {
	if podName == "" {
		return fmt.Errorf("pod name cannot be empty")
	}
	if len(podName) > 253 {
		return fmt.Errorf("pod name too long (max 253 characters)")
	}
	// TODO: 添加更多验证规则
	return nil
}

// ValidateContainerName 验证容器名称
func ValidateContainerName(containerName string) error {
	if containerName == "" {
		return nil // 容器名称可以为空，表示使用第一个容器
	}
	if len(containerName) > 63 {
		return fmt.Errorf("container name too long (max 63 characters)")
	}
	// TODO: 添加更多验证规则
	return nil
}

// EnsureOutputDir 确保输出目录存在
func EnsureOutputDir(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// SaveToFile 保存数据到文件
func SaveToFile(data []byte, outputPath string) error {
	// 确保输出目录存在
	if err := EnsureOutputDir(outputPath); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 写入文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data to file: %w", err)
	}

	return nil
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// 确保目标目录存在
	if err := EnsureOutputDir(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// ParseDuration 解析持续时间字符串
func ParseDuration(duration string) (time.Duration, error) {
	if duration == "" {
		return 30 * time.Second, nil // 默认30秒
	}
	return time.ParseDuration(duration)
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// SanitizeFilename 清理文件名
func SanitizeFilename(filename string) string {
	// 替换不安全的字符
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(filename)
}

// GetFileSize 获取文件大小
func GetFileSize(filepath string) (int64, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FileExists 检查文件是否存在
func FileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

// IsValidImageName 验证镜像名称
func IsValidImageName(image string) bool {
	if image == "" {
		return false
	}
	// TODO: 添加更严格的镜像名称验证
	return true
}

// ExtractContainerID 从容器ID字符串中提取实际ID
func ExtractContainerID(containerID string) string {
	if strings.Contains(containerID, "://") {
		parts := strings.SplitN(containerID, "://", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return containerID
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}