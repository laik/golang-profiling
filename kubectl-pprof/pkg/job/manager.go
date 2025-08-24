package job

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/withlin/kubectl-pprof/internal/types"
	"github.com/withlin/kubectl-pprof/pkg/config"
)

// Manager simplified Job manager
type Manager struct {
	k8sConfig *config.KubernetesConfig
	cleaner   *JobCleaner
}

// NewManager creates a new Job manager
func NewManager(k8sConfig *config.KubernetesConfig) (*Manager, error) {
	// Create cleaner
	cleaner := NewJobCleaner(k8sConfig.Clientset, nil, nil)

	return &Manager{
		k8sConfig: k8sConfig,
		cleaner:   cleaner,
	}, nil
}

// CreateProfilingJobWithMonitoring creates a profiling Job and monitors execution
func (m *Manager) CreateProfilingJobWithMonitoring(ctx context.Context, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) (*types.ProfileResult, error) {
	// Generate Job name
	jobName := fmt.Sprintf("kubectl-pprof-%d", time.Now().Unix())

	// Create Job
	job := m.buildJobSpec(jobName, cfg, opts, target)
	_, err := m.k8sConfig.Clientset.BatchV1().Jobs(cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Wait for Job completion, decide whether to print logs based on PrintLogs parameter
	var status *types.JobStatus
	if opts.PrintLogs {
		status, err = m.WaitForCompletionWithLogs(ctx, jobName, cfg.Namespace, 5*time.Minute)
	} else {
		status, err = m.WaitForCompletion(ctx, jobName, cfg.Namespace, 5*time.Minute)
	}
	if err != nil {
		return nil, fmt.Errorf("job execution failed: %w", err)
	}

	// Extract flame graph content from logs (temporarily commented out to simplify implementation)
	// flameGraphData, err := m.extractFlameGraphFromLogs(ctx, jobName, cfg.Namespace)
	// if err != nil {
	//	return nil, fmt.Errorf("failed to extract flamegraph from logs: %w", err)
	// }

	// Clean up Job
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m.DeleteJob(cleanupCtx, jobName, cfg.Namespace)
	}()

	return &types.ProfileResult{
		JobName:   jobName,
		JobStatus: status,
		Success:   status.Phase == types.JobPhaseSucceeded,
	}, nil
}

// extractFlameGraphFromLogs extracts flame graph content from Pod logs
func (m *Manager) extractFlameGraphFromLogs(ctx context.Context, jobName, namespace string) ([]byte, error) {
	// Get Pods associated with the Job
	pods, err := m.k8sConfig.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}

	pod := pods.Items[0]

	// Get Pod logs
	req := m.k8sConfig.Clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "profiler",
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	// Parse logs to find flame graph content
	scanner := bufio.NewScanner(logs)
	var flameGraphContent strings.Builder
	inFlameGraph := false

	// Define flame graph start and end markers
	flameGraphStartPattern := regexp.MustCompile(`^FLAMEGRAPH_START:(.*)$`)
	flameGraphEndPattern := regexp.MustCompile(`^FLAMEGRAPH_END$`)

	for scanner.Scan() {
		line := scanner.Text()

		if matches := flameGraphStartPattern.FindStringSubmatch(line); matches != nil {
			// Found flame graph start marker
			inFlameGraph = true
			if len(matches) > 1 && matches[1] != "" {
				// If start marker contains content, add to flame graph
				flameGraphContent.WriteString(matches[1])
			}
			continue
		}

		if flameGraphEndPattern.MatchString(line) {
			// Found flame graph end marker
			inFlameGraph = false
			break
		}

		if inFlameGraph {
			// In flame graph content area, collect all lines
			flameGraphContent.WriteString(line)
			flameGraphContent.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading logs: %w", err)
	}

	if flameGraphContent.Len() == 0 {
		return nil, fmt.Errorf("no flamegraph content found in logs")
	}

	// Decode base64 content and decompress gzip
	content := strings.TrimSpace(flameGraphContent.String())
	if content == "" {
		return nil, fmt.Errorf("empty flamegraph content")
	}

	// Decode base64
	decodedData, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 content: %w", err)
	}

	// Decompress gzip
	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Read decompressed content
	decompressedData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip content: %w", err)
	}

	return decompressedData, nil
}

// buildJobSpec builds Job specification
func (m *Manager) buildJobSpec(jobName string, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) *batchv1.Job {
	// Build profiling script
	script := m.buildAdvancedProfilingScript(target, cfg)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app": "kubectl-pprof",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &[]int32{0}[0],
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kubectl-pprof",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					HostPID:       true,
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": target.NodeName,
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "profiler",
							Image:           cfg.Image,
							Command:         []string{"/bin/sh"},
							Args:            []string{"-c", script},
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &[]bool{true}[0],
								RunAsUser:  &[]int64{0}[0],
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SYS_ADMIN",
										"SYS_RESOURCE",
										"SYS_PTRACE",
										"BPF",
										"PERFMON",
									},
								},
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
								{
									Name:      "containerd-sock",
									MountPath: "/run/containerd/containerd.sock",
									ReadOnly:  true,
								},
								{
									Name:      "crictl-bin",
									MountPath: "/usr/local/bin/crictl",
									ReadOnly:  true,
								},
							},
						},
					},
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
						{
							Name: "containerd-sock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/containerd/containerd.sock",
								},
							},
						},
						{
							Name: "crictl-bin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/usr/bin/crictl",
								},
							},
						},
					},
				},
			},
		},
	}

	return job
}

// buildProfilingArgs builds profiling arguments
func (m *Manager) buildProfilingArgs(cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) []string {
	args := []string{
		"--pid", fmt.Sprintf("%d", target.PID),
		"--duration", fmt.Sprintf("%.0f", cfg.Duration.Seconds()),
	}

	if cfg.GoOptions != nil && cfg.GoOptions.Frequency > 0 {
		args = append(args, "--frequency", fmt.Sprintf("%d", cfg.GoOptions.Frequency))
	}

	if cfg.GoOptions != nil && cfg.GoOptions.Width > 0 {
		args = append(args, "--width", fmt.Sprintf("%d", cfg.GoOptions.Width))
	}

	if cfg.GoOptions != nil && cfg.GoOptions.Height > 0 {
		args = append(args, "--height", fmt.Sprintf("%d", cfg.GoOptions.Height))
	}

	return args
}

// buildAdvancedProfilingScript builds advanced profiling script
func (m *Manager) buildAdvancedProfilingScript(target *types.TargetInfo, cfg *types.ProfileConfig) string {
	// Convert duration to seconds
	durationSeconds := int(cfg.Duration.Seconds())

	return fmt.Sprintf(`		
		# Get target container ID (using grep to match container name)
		CONTAINER_ID=$(crictl --runtime-endpoint unix:///run/containerd/containerd.sock ps | grep -w "%s" | awk '{print $1}' | head -1)
		if [ -z "$CONTAINER_ID" ]; then
			echo "Error: Container %s not found"
			echo "Available containers:"
			crictl --runtime-endpoint unix:///run/containerd/containerd.sock ps
			exit 1
		fi
		
		echo "Found container ID: $CONTAINER_ID"
		
		# Get container PID
		CONTAINER_PID=$(crictl --runtime-endpoint unix:///run/containerd/containerd.sock inspect "$CONTAINER_ID" | grep '"pid"' | head -1 | awk '{print $2}' | tr -d ',')
		if [ -z "$CONTAINER_PID" ]; then
			echo "Error: Cannot get PID for container $CONTAINER_ID"
			exit 1
		fi
		
		echo "Found target container PID: $CONTAINER_PID"
		
		# Check if PID exists
		if [ ! -d "/host/proc/$CONTAINER_PID" ]; then
			echo "Error: Process $CONTAINER_PID not found in /host/proc"
			echo "Available processes:"
			ls /host/proc/ | grep '^[0-9]*$' | head -10
			exit 1
		fi
		
		# Use nsenter to enter target container namespace and run profiling
		# Need to use host proc filesystem
		PROC_PATH="/host/proc/$CONTAINER_PID"
		if [ ! -d "$PROC_PATH/ns" ]; then
			echo "Error: Cannot access namespace files at $PROC_PATH/ns"
			echo "Available proc entries:"
			ls /host/proc/ | grep '^[0-9]*$' | head -5
			exit 1
		fi
		
		# Run golang-profiling directly on host, specifying target PID
		# Set PROC_ROOT environment variable to point to host proc filesystem
		export PROC_ROOT=/host/proc
		echo "Starting golang-profiling with arguments: --pid $CONTAINER_PID --duration %d --output /tmp/profile.svg"
		/usr/local/bin/golang-profiling --pid $CONTAINER_PID --duration %d --output /tmp/profile.svg
		PROFILE_EXIT_CODE=$?
		echo "golang-profiling exit code: $PROFILE_EXIT_CODE"
		if [ $PROFILE_EXIT_CODE -eq 0 ]; then
			echo "Profiling completed successfully"
			ls -la /tmp/profile.svg
			
			# Output flame graph content to logs (using gzip compression and base64 encoding)
			echo -n "FLAMEGRAPH_START:"
			gzip -c /tmp/profile.svg | base64 -w 0
			echo ""
			echo "FLAMEGRAPH_END"
			
			# Create completion marker file
			echo "PROFILING_COMPLETED" > /tmp/profiling_done
			echo "Profiling completed and flamegraph output to logs"
		else
			echo "Profiling failed with exit code: $PROFILE_EXIT_CODE"
		fi
	`, target.ContainerName, target.ContainerName, durationSeconds, durationSeconds)
}

// WaitForCompletion waits for Job completion
func (m *Manager) WaitForCompletion(ctx context.Context, jobName string, namespace string, timeout time.Duration) (*types.JobStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var finalStatus *types.JobStatus
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		status, err := m.GetJobStatus(ctx, jobName, namespace)
		if err != nil {
			return false, err
		}

		finalStatus = status
		switch status.Phase {
		case types.JobPhaseSucceeded, types.JobPhaseFailed:
			return true, nil
		default:
			return false, nil
		}
	})

	if err != nil {
		return nil, err
	}

	return finalStatus, nil
}

// WaitForCompletionWithLogs waits for Job completion and prints logs in real time
func (m *Manager) WaitForCompletionWithLogs(ctx context.Context, jobName string, namespace string, timeout time.Duration) (*types.JobStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for Pod to start
	var podName string
	for i := 0; i < 30; i++ {
		pods, err := m.k8sConfig.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		})
		if err == nil && len(pods.Items) > 0 {
			podName = pods.Items[0].Name
			break
		}
		time.Sleep(1 * time.Second)
	}

	if podName == "" {
		return nil, fmt.Errorf("failed to find pod for job %s", jobName)
	}

	fmt.Printf("📋 Streaming logs from pod %s...\n", podName)

	// Start log streaming
	go m.streamPodLogs(ctx, podName, namespace)

	// Wait for Job completion
	var finalStatus *types.JobStatus
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		status, err := m.GetJobStatus(ctx, jobName, namespace)
		if err != nil {
			return false, err
		}

		finalStatus = status
		switch status.Phase {
		case types.JobPhaseSucceeded, types.JobPhaseFailed:
			return true, nil
		default:
			return false, nil
		}
	})

	if err != nil {
		return nil, err
	}

	fmt.Println("📋 Log streaming completed.")
	return finalStatus, nil
}

// streamPodLogs streams Pod logs
func (m *Manager) streamPodLogs(ctx context.Context, podName, namespace string) {
	// Wait for Pod to enter Running state
	for i := 0; i < 60; i++ {
		pod, err := m.k8sConfig.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil && pod.Status.Phase == corev1.PodRunning {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}

	// Get log stream
	req := m.k8sConfig.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "profiler",
		Follow:    true,
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to stream logs: %v\n", err)
		return
	}
	defer logs.Close()

	// Read and print logs
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Println(scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Warning: error reading logs: %v\n", err)
	}
}

// GetJobStatus gets Job status
func (m *Manager) GetJobStatus(ctx context.Context, jobName string, namespace string) (*types.JobStatus, error) {
	job, err := m.k8sConfig.Clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	status := &types.JobStatus{
		JobName:   job.Name,
		Namespace: job.Namespace,
		Phase:     types.JobPhaseRunning,
	}

	if job.Status.Succeeded > 0 {
		status.Phase = types.JobPhaseSucceeded
	} else if job.Status.Failed > 0 {
		status.Phase = types.JobPhaseFailed
	}

	return status, nil
}

// DeleteJob deletes Job
func (m *Manager) DeleteJob(ctx context.Context, jobName string, namespace string) error {
	propagationPolicy := metav1.DeletePropagationForeground
	return m.k8sConfig.Clientset.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}

// ExtractFlameGraphFromLogs public method for extracting flame graph from logs
func (m *Manager) ExtractFlameGraphFromLogs(ctx context.Context, jobName, namespace string) ([]byte, error) {
	return m.extractFlameGraphFromLogs(ctx, jobName, namespace)
}

// Test methods retained for compatibility
func (m *Manager) BuildProfilingArgsForTest(cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) []string {
	return m.buildProfilingArgs(cfg, opts, target)
}

func (m *Manager) BuildProfilingScriptForTest(target *types.TargetInfo, cfg *types.ProfileConfig) string {
	return m.buildAdvancedProfilingScript(target, cfg)
}

func (m *Manager) BuildJobSpecForTest(jobName string, cfg *types.ProfileConfig, opts *types.ProfileOptions, target *types.TargetInfo) *batchv1.Job {
	return m.buildJobSpec(jobName, cfg, opts, target)
}
