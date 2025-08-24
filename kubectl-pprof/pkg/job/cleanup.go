package job

import (
	"context"
	"fmt"
	"log"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CleanupConfig 清理配置
type CleanupConfig struct {
	// 自动清理延迟时间
	AutoCleanupDelay time.Duration
	// 最大 Job 保留时间
	MaxJobRetention time.Duration
	// 清理检查间隔
	CleanupInterval time.Duration
	// 是否启用自动清理
	EnableAutoCleanup bool
	// 清理失败的 Job
	CleanupFailedJobs bool
	// 清理成功的 Job
	CleanupSuccessfulJobs bool
}

// DefaultCleanupConfig 默认清理配置
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		AutoCleanupDelay:      30 * time.Second,
		MaxJobRetention:       24 * time.Hour,
		CleanupInterval:       5 * time.Minute,
		EnableAutoCleanup:     true,
		CleanupFailedJobs:     true,
		CleanupSuccessfulJobs: true,
	}
}

// JobCleaner Job 清理器
type JobCleaner struct {
	client kubernetes.Interface
	config *CleanupConfig
	logger *log.Logger
	stopCh chan struct{}
}

// NewJobCleaner 创建新的 Job 清理器
func NewJobCleaner(client kubernetes.Interface, config *CleanupConfig, logger *log.Logger) *JobCleaner {
	if config == nil {
		config = DefaultCleanupConfig()
	}

	return &JobCleaner{
		client: client,
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start 启动自动清理
func (jc *JobCleaner) Start(ctx context.Context) {
	if !jc.config.EnableAutoCleanup {
		jc.logf("Auto cleanup is disabled")
		return
	}

	jc.logf("Starting job cleaner with interval: %v", jc.config.CleanupInterval)

	ticker := time.NewTicker(jc.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			jc.logf("Job cleaner stopped due to context cancellation")
			return
		case <-jc.stopCh:
			jc.logf("Job cleaner stopped")
			return
		case <-ticker.C:
			if err := jc.cleanupExpiredJobs(ctx); err != nil {
				jc.logf("Error during cleanup: %v", err)
			}
		}
	}
}

// Stop 停止自动清理
func (jc *JobCleaner) Stop() {
	close(jc.stopCh)
}

// CleanupJob 清理指定的 Job
func (jc *JobCleaner) CleanupJob(ctx context.Context, jobName, namespace string) error {
	jc.logf("Cleaning up job: %s in namespace: %s", jobName, namespace)

	// 删除 Job（前台删除，确保 Pod 也被删除）
	deletePolicy := metav1.DeletePropagationForeground
	err := jc.client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})

	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", jobName, err)
	}

	jc.logf("Successfully cleaned up job: %s", jobName)
	return nil
}

// CleanupJobAfterDelay 延迟清理 Job
func (jc *JobCleaner) CleanupJobAfterDelay(ctx context.Context, jobName, namespace string) {
	go func() {
		timer := time.NewTimer(jc.config.AutoCleanupDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			jc.logf("Cleanup cancelled for job: %s", jobName)
			return
		case <-timer.C:
			if err := jc.CleanupJob(ctx, jobName, namespace); err != nil {
				jc.logf("Failed to cleanup job %s after delay: %v", jobName, err)
			}
		}
	}()
}

// cleanupExpiredJobs 清理过期的 Job
func (jc *JobCleaner) cleanupExpiredJobs(ctx context.Context) error {
	// 获取所有命名空间的 Job
	jobs, err := jc.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{
		LabelSelector: "app=kubectl-pprof", // 只清理我们创建的 Job
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	now := time.Now()
	cleanedCount := 0

	for _, job := range jobs.Items {
		if jc.shouldCleanupJob(&job, now) {
			if err := jc.CleanupJob(ctx, job.Name, job.Namespace); err != nil {
				jc.logf("Failed to cleanup expired job %s: %v", job.Name, err)
				continue
			}
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		jc.logf("Cleaned up %d expired jobs", cleanedCount)
	}

	return nil
}

// shouldCleanupJob 判断是否应该清理 Job
func (jc *JobCleaner) shouldCleanupJob(job *batchv1.Job, now time.Time) bool {
	// 检查 Job 年龄
	age := now.Sub(job.CreationTimestamp.Time)
	if age > jc.config.MaxJobRetention {
		return true
	}

	// 检查 Job 状态
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobComplete:
			if condition.Status == corev1.ConditionTrue && jc.config.CleanupSuccessfulJobs {
				// 成功的 Job，检查完成时间
				completionAge := now.Sub(condition.LastTransitionTime.Time)
				return completionAge > jc.config.AutoCleanupDelay
			}
		case batchv1.JobFailed:
			if condition.Status == corev1.ConditionTrue && jc.config.CleanupFailedJobs {
				// 失败的 Job，检查失败时间
				failureAge := now.Sub(condition.LastTransitionTime.Time)
				return failureAge > jc.config.AutoCleanupDelay
			}
		}
	}

	return false
}

// GetJobStatus 获取 Job 状态
func (jc *JobCleaner) GetJobStatus(ctx context.Context, jobName, namespace string) (*batchv1.Job, error) {
	return jc.client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
}

// WaitForJobCompletion 等待 Job 完成
func (jc *JobCleaner) WaitForJobCompletion(ctx context.Context, jobName, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for job %s to complete", jobName)
		case <-ticker.C:
			job, err := jc.GetJobStatus(ctx, jobName, namespace)
			if err != nil {
				return fmt.Errorf("failed to get job status: %w", err)
			}

			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					jc.logf("Job %s completed successfully", jobName)
					return nil
				}
				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					return fmt.Errorf("job %s failed: %s", jobName, condition.Message)
				}
			}
		}
	}
}

// logf 记录日志
func (jc *JobCleaner) logf(format string, args ...interface{}) {
	if jc.logger != nil {
		jc.logger.Printf("[JobCleaner] "+format, args...)
	}
}