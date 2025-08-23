package job

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/withlin/kubectl-pprof/internal/types"
	"github.com/withlin/kubectl-pprof/pkg/config"
)

// Manager Job管理器
type Manager struct {
	k8sConfig *config.KubernetesConfig
}

// NewManager 创建新的Job管理器
func NewManager(k8sConfig *config.KubernetesConfig) (*Manager, error) {
	return &Manager{
		k8sConfig: k8sConfig,
	}, nil
}

// CreateProfilingJob 创建性能分析Job
func (m *Manager) CreateProfilingJob(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) (string, error) {
	// 生成Job名称
	jobName := fmt.Sprintf("%s-%d", cfg.JobName, time.Now().Unix())

	// 创建Job规范
	job := m.buildJobSpec(jobName, cfg, opts, target)

	// 创建Job
	_, err := m.k8sConfig.Clientset.BatchV1().Jobs(cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create job: %w", err)
	}

	return jobName, nil
}

// buildJobSpec 构建Job规范
func (m *Manager) buildJobSpec(jobName string, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) *batchv1.Job {
	// 构建命令参数
	args := m.buildProfilingArgs(cfg, opts, target)

	// 构建容器规范
	container := corev1.Container{
		Name:  "profiler",
		Image: cfg.Image,
		Args:  args,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &cfg.Privileged,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "proc",
				MountPath: "/host/proc",
				ReadOnly:  true,
			},
			{
				Name:      "sys",
				MountPath: "/host/sys",
				ReadOnly:  true,
			},
		},
	}

	// 设置资源限制
	if cfg.ResourceLimits != nil {
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{},
		}
		if cfg.ResourceLimits.CPU != "" {
			// TODO: 解析CPU限制
		}
		if cfg.ResourceLimits.Memory != "" {
			// TODO: 解析内存限制
		}
	}

	// 设置环境变量
	for k, v := range cfg.EnvVars {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	// 构建Pod规范
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{container},
		Volumes: []corev1.Volume{
			{
				Name: "proc",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/proc",
					},
				},
			},
			{
				Name: "sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys",
					},
				},
			},
		},
		NodeSelector: map[string]string{},
	}

	// 设置节点选择器
	if cfg.NodeName != "" {
		podSpec.NodeSelector["kubernetes.io/hostname"] = cfg.NodeName
	} else if target.NodeName != "" {
		podSpec.NodeSelector["kubernetes.io/hostname"] = target.NodeName
	}

	// 设置PID命名空间共享
	podSpec.ShareProcessNamespace = &[]bool{true}[0]

	// 构建Job
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app":                          "kubectl-pprof",
				"kubectl-pprof/job":            jobName,
				"kubectl-pprof/target-pod":     target.PodName,
				"kubectl-pprof/target-container": target.ContainerName,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
			BackoffLimit: &[]int32{0}[0],
		},
	}
}

// buildProfilingArgs 构建分析参数
func (m *Manager) buildProfilingArgs(cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) []string {
	args := []string{
		"--type", cfg.ProfileType,
		"--duration", cfg.Duration.String(),
		"--output", "/tmp/profile.svg",
		"--target-pid", "1", // TODO: 获取实际的目标PID
	}

	// 添加输出格式
	if opts.OutputFormat != "" {
		args = append(args, "--format", opts.OutputFormat)
	}

	// 添加采样率
	if opts.SampleRate > 0 {
		args = append(args, "--sample-rate", fmt.Sprintf("%d", opts.SampleRate))
	}

	// 添加栈深度
	if opts.StackDepth > 0 {
		args = append(args, "--stack-depth", fmt.Sprintf("%d", opts.StackDepth))
	}

	// 添加过滤器
	if opts.FilterPattern != "" {
		args = append(args, "--filter", opts.FilterPattern)
	}

	// 添加忽略模式
	if opts.IgnorePattern != "" {
		args = append(args, "--ignore", opts.IgnorePattern)
	}

	// 添加额外参数
	args = append(args, cfg.ExtraArgs...)

	return args
}

// WaitForCompletion 等待Job完成
func (m *Manager) WaitForCompletion(ctx context.Context, jobName string, timeout time.Duration) (*types.JobStatus, error) {
	// 创建超时上下文
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 等待Job完成
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		job, err := m.k8sConfig.Clientset.BatchV1().Jobs(m.k8sConfig.Namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// 检查Job状态
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return false, fmt.Errorf("job failed: %s", condition.Message)
			}
		}

		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("job did not complete: %w", err)
	}

	// 获取最终状态
	return m.GetJobStatus(ctx, jobName)
}

// GetJobStatus 获取Job状态
func (m *Manager) GetJobStatus(ctx context.Context, jobName string) (*types.JobStatus, error) {
	job, err := m.k8sConfig.Clientset.BatchV1().Jobs(m.k8sConfig.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	status := &types.JobStatus{
		JobName: jobName,
		Phase:   types.JobPhaseRunning,
	}

	// 设置开始时间
	if job.Status.StartTime != nil {
		status.StartTime = &job.Status.StartTime.Time
	}

	// 设置完成时间
	if job.Status.CompletionTime != nil {
		status.EndTime = &job.Status.CompletionTime.Time
	}

	// 检查Job状态
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			status.Phase = types.JobPhaseSucceeded
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			status.Phase = types.JobPhaseFailed
			status.Message = condition.Message
		}
	}

	return status, nil
}

// GetJobOutput 获取Job输出
func (m *Manager) GetJobOutput(ctx context.Context, jobName string) ([]byte, error) {
	// 获取Job关联的Pod
	pods, err := m.k8sConfig.Clientset.CoreV1().Pods(m.k8sConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list job pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}

	// 使用第一个Pod
	pod := pods.Items[0]
	if pod.Status.Phase != corev1.PodSucceeded {
		return nil, fmt.Errorf("pod %s is not in succeeded state: %s", pod.Name, pod.Status.Phase)
	}

	// 方法1: 通过kubectl cp获取文件
	data, err := m.copyFileFromPod(ctx, pod.Name, "/tmp/profile.svg")
	if err != nil {
		// 方法2: 通过Pod日志获取base64编码的文件内容
		return m.getOutputFromLogs(ctx, pod.Name)
	}

	return data, nil
}

// ListJobs 列出Job
func (m *Manager) ListJobs(ctx context.Context, namespace string) ([]*types.JobStatus, error) {
	jobs, err := m.k8sConfig.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=kubectl-pprof",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var result []*types.JobStatus
	for _, job := range jobs.Items {
		status, err := m.GetJobStatus(ctx, job.Name)
		if err != nil {
			continue
		}
		result = append(result, status)
	}

	return result, nil
}

// DeleteJob 删除Job
func (m *Manager) DeleteJob(ctx context.Context, jobName string) error {
	propagationPolicy := metav1.DeletePropagationForeground
	return m.k8sConfig.Clientset.BatchV1().Jobs(m.k8sConfig.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}

// copyFileFromPod 从Pod中复制文件
func (m *Manager) copyFileFromPod(ctx context.Context, podName, filePath string) ([]byte, error) {
	// 执行cat命令读取文件内容
	cmd := []string{"cat", filePath}
	req := m.k8sConfig.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(m.k8sConfig.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd,
		Stdout:  true,
		Stderr:  true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(m.k8sConfig.Config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w, stderr: %s", err, stderr.String())
	}

	if stderr.Len() > 0 {
		return nil, fmt.Errorf("command failed with stderr: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}

// getOutputFromLogs 从Pod日志获取输出
func (m *Manager) getOutputFromLogs(ctx context.Context, podName string) ([]byte, error) {
	// 获取Pod日志
	req := m.k8sConfig.Clientset.CoreV1().Pods(m.k8sConfig.Namespace).GetLogs(podName, &corev1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	// 读取日志内容
	logData, err := io.ReadAll(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	// 查找base64编码的文件内容
	// 假设golang-profiling工具会在日志中输出: "FLAMEGRAPH_DATA: <base64_encoded_data>"
	logStr := string(logData)
	lines := strings.Split(logStr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "FLAMEGRAPH_DATA: ") {
			base64Data := strings.TrimPrefix(line, "FLAMEGRAPH_DATA: ")
			data, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 data: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("no flamegraph data found in logs")
}