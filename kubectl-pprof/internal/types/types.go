package types

import (
	"fmt"
	"time"
)

// ProfileConfig 性能分析配置
type ProfileConfig struct {
	// 目标信息
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`

	// 分析参数
	Duration    time.Duration `json:"duration"`
	ProfileType string        `json:"profileType"` // cpu, memory, goroutine, block
	OutputPath  string        `json:"outputPath"`

	// Job配置
	JobName       string        `json:"jobName"`
	Image         string        `json:"image"`
	NodeName      string        `json:"nodeName,omitempty"`
	Timeout       time.Duration `json:"timeout"`
	Cleanup       bool          `json:"cleanup"`
	Privileged    bool          `json:"privileged"`

	// 高级选项
	ExtraArgs     []string          `json:"extraArgs,omitempty"`
	EnvVars       map[string]string `json:"envVars,omitempty"`
	ResourceLimits *ResourceLimits   `json:"resourceLimits,omitempty"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// TargetInfo 目标容器信息
type TargetInfo struct {
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	NodeName      string `json:"nodeName"`
	PodUID        string `json:"podUID"`
	ContainerID   string `json:"containerID"`
	PID           int32  `json:"pid,omitempty"`
	Status        string `json:"status"`
	Image         string `json:"image"`
	Command       []string `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	Pod           interface{} `json:"pod,omitempty"` // *corev1.Pod
	Container     interface{} `json:"container,omitempty"` // *corev1.Container
	NodeInfo      *NodeInfo `json:"nodeInfo,omitempty"`
	RuntimeInfo   *RuntimeInfo `json:"runtimeInfo,omitempty"`
}

// JobStatus Job执行状态
type JobStatus struct {
	JobName   string             `json:"jobName"`
	Namespace string             `json:"namespace"`
	Phase     JobPhase           `json:"phase"`
	StartTime *time.Time         `json:"startTime,omitempty"`
	EndTime   *time.Time         `json:"endTime,omitempty"`
	Message   string             `json:"message,omitempty"`
	PodName   string             `json:"podName,omitempty"`
	Conditions []JobCondition    `json:"conditions,omitempty"`
}

// JobPhase Job阶段
type JobPhase string

const (
	JobPhasePending   JobPhase = "Pending"
	JobPhaseRunning   JobPhase = "Running"
	JobPhaseSucceeded JobPhase = "Succeeded"
	JobPhaseFailed    JobPhase = "Failed"
	JobPhaseTimeout   JobPhase = "Timeout"
)

// JobCondition Job条件
type JobCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// NodeCondition 节点条件
type NodeCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// ProfileResult 分析结果
type ProfileResult struct {
	Config     *ProfileConfig `json:"config"`
	JobStatus  *JobStatus     `json:"jobStatus"`
	OutputPath string         `json:"outputPath"`
	FileSize   int64          `json:"fileSize"`
	Duration   time.Duration  `json:"duration"`
	Samples    int64          `json:"samples,omitempty"`
	Error      string         `json:"error,omitempty"`
	JobName    string         `json:"jobName"`
	Success    bool           `json:"success"`
}

// ContainerRuntime 容器运行时类型
type ContainerRuntime string

const (
	RuntimeContainerd ContainerRuntime = "containerd"
	RuntimeDocker     ContainerRuntime = "docker"
	RuntimeCRIO       ContainerRuntime = "cri-o"
)

// RuntimeInfo 运行时信息
type RuntimeInfo struct {
	Type            ContainerRuntime `json:"type"`
	Version         string           `json:"version"`
	SocketPath      string           `json:"socketPath"`
	SupportsSharing bool             `json:"supportsSharing"`
	Runtime         ContainerRuntime `json:"runtime"`
	ContainerID     string           `json:"containerID"`
	ImageID         string           `json:"imageID"`
	PID             int              `json:"pid"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	Name            string                `json:"name"`
	Labels          map[string]string     `json:"labels"`
	Annotations     map[string]string     `json:"annotations"`
	Conditions      []NodeCondition       `json:"conditions"`
	Capacity        map[string]string     `json:"capacity"`
	Allocatable     map[string]string     `json:"allocatable"`
	RuntimeInfo     *RuntimeInfo          `json:"runtimeInfo,omitempty"`
	KubeletVersion  string                `json:"kubeletVersion"`
	OperatingSystem string                `json:"operatingSystem"`
	Architecture    string                `json:"architecture"`
	KernelVersion   string                `json:"kernelVersion"`
	OSImage         string                `json:"osImage"`
}

// ProfileOptions 分析选项
type ProfileOptions struct {
	// 基础选项
	CPUProfile     bool `json:"cpuProfile"`
	MemoryProfile  bool `json:"memoryProfile"`
	GoroutineProfile bool `json:"goroutineProfile"`
	BlockProfile   bool `json:"blockProfile"`
	MutexProfile   bool `json:"mutexProfile"`

	// 输出选项
	FlameGraph     bool   `json:"flameGraph"`
	RawData        bool   `json:"rawData"`
	JSONReport     bool   `json:"jsonReport"`
	OutputFormat   string `json:"outputFormat"` // svg, png, pdf, json

	// 高级选项
	SampleRate     int    `json:"sampleRate,omitempty"`
	StackDepth     int    `json:"stackDepth,omitempty"`
	FilterPattern  string `json:"filterPattern,omitempty"`
	IgnorePattern  string `json:"ignorePattern,omitempty"`
}

// ErrorCode 错误代码
type ErrorCode string

const (
	ErrCodePodNotFound        ErrorCode = "POD_NOT_FOUND"
	ErrCodeContainerNotFound  ErrorCode = "CONTAINER_NOT_FOUND"
	ErrCodePodNotRunning      ErrorCode = "POD_NOT_RUNNING"
	ErrCodeInsufficientPerms  ErrorCode = "INSUFFICIENT_PERMISSIONS"
	ErrCodeJobCreationFailed  ErrorCode = "JOB_CREATION_FAILED"
	ErrCodeJobTimeout         ErrorCode = "JOB_TIMEOUT"
	ErrCodeJobFailed          ErrorCode = "JOB_FAILED"
	ErrCodeResultNotFound     ErrorCode = "RESULT_NOT_FOUND"
	ErrCodeInvalidConfig      ErrorCode = "INVALID_CONFIG"
	ErrCodeRuntimeError       ErrorCode = "RUNTIME_ERROR"
)

// ProfileError 分析错误
type ProfileError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Cause   error     `json:"-"`
}

func (e *ProfileError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ProfileError) Unwrap() error {
	return e.Cause
}

// NewProfileError 创建分析错误
func NewProfileError(code ErrorCode, message string, cause error) *ProfileError {
	return &ProfileError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}