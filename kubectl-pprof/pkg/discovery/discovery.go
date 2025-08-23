package discovery

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/withlin/kubectl-pprof/internal/types"
	"github.com/withlin/kubectl-pprof/pkg/config"
)

// Discovery 容器发现服务
type Discovery struct {
	k8sConfig *config.KubernetesConfig
}

// NewDiscovery 创建新的发现服务
func NewDiscovery(k8sConfig *config.KubernetesConfig) (*Discovery, error) {
	return &Discovery{
		k8sConfig: k8sConfig,
	}, nil
}

// FindPod 查找Pod
func (d *Discovery) FindPod(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	pod, err := d.k8sConfig.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	// 验证Pod状态
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod %s/%s is not running (phase: %s)", namespace, podName, pod.Status.Phase)
	}

	return pod, nil
}

// FindContainer 查找容器
func (d *Discovery) FindContainer(pod *corev1.Pod, containerName string) (*corev1.Container, error) {
	// 如果没有指定容器名，使用第一个容器
	if containerName == "" {
		if len(pod.Spec.Containers) == 0 {
			return nil, fmt.Errorf("no containers found in pod %s/%s", pod.Namespace, pod.Name)
		}
		return &pod.Spec.Containers[0], nil
	}

	// 查找指定的容器
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return &container, nil
		}
	}

	return nil, fmt.Errorf("container %s not found in pod %s/%s", containerName, pod.Namespace, pod.Name)
}

// GetNodeInfo 获取节点信息
func (d *Discovery) GetNodeInfo(ctx context.Context, nodeName string) (*types.NodeInfo, error) {
	node, err := d.k8sConfig.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// 转换节点条件
	conditions := make([]types.NodeCondition, len(node.Status.Conditions))
	for i, cond := range node.Status.Conditions {
		conditions[i] = types.NodeCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			LastTransitionTime: cond.LastTransitionTime.Time,
			Reason:             cond.Reason,
			Message:            cond.Message,
		}
	}

	// 转换资源信息
	capacity := make(map[string]string)
	for k, v := range node.Status.Capacity {
		capacity[string(k)] = v.String()
	}

	allocatable := make(map[string]string)
	for k, v := range node.Status.Allocatable {
		allocatable[string(k)] = v.String()
	}

	return &types.NodeInfo{
		Name:        node.Name,
		Labels:      node.Labels,
		Annotations: node.Annotations,
		Conditions:  conditions,
		Capacity:    capacity,
		Allocatable: allocatable,
		KernelVersion: node.Status.NodeInfo.KernelVersion,
		OSImage:     node.Status.NodeInfo.OSImage,
		Architecture: node.Status.NodeInfo.Architecture,
	}, nil
}

// GetRuntimeInfo 获取运行时信息
func (d *Discovery) GetRuntimeInfo(ctx context.Context, pod *corev1.Pod, container *corev1.Container) (*types.RuntimeInfo, error) {
	// 检测容器运行时
	runtime := d.detectContainerRuntime(pod)

	// 获取容器状态
	containerStatus := d.getContainerStatus(pod, container.Name)
	if containerStatus == nil {
		return nil, fmt.Errorf("container %s status not found", container.Name)
	}

	return &types.RuntimeInfo{
		Runtime:     runtime,
		ContainerID: containerStatus.ContainerID,
		ImageID:     containerStatus.ImageID,
		PID:         0, // TODO: 获取容器PID
	}, nil
}

// detectContainerRuntime 检测容器运行时
func (d *Discovery) detectContainerRuntime(pod *corev1.Pod) types.ContainerRuntime {
	// 从容器状态中检测运行时
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.ContainerID != "" {
			if len(containerStatus.ContainerID) > 11 && containerStatus.ContainerID[:11] == "containerd:" {
				return types.RuntimeContainerd
			}
			if len(containerStatus.ContainerID) > 9 && containerStatus.ContainerID[:9] == "docker://" {
				return types.RuntimeDocker
			}
			if len(containerStatus.ContainerID) > 6 && containerStatus.ContainerID[:6] == "cri-o:" {
				return types.RuntimeCRIO
			}
		}
	}

	// 默认假设是containerd
	return types.RuntimeContainerd
}

// getContainerStatus 获取容器状态
func (d *Discovery) getContainerStatus(pod *corev1.Pod, containerName string) *corev1.ContainerStatus {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return &status
		}
	}
	return nil
}

// ValidateTarget 验证目标容器
func (d *Discovery) ValidateTarget(ctx context.Context, namespace, podName, containerName string) error {
	// 查找Pod
	pod, err := d.FindPod(ctx, namespace, podName)
	if err != nil {
		return err
	}

	// 查找容器
	_, err = d.FindContainer(pod, containerName)
	if err != nil {
		return err
	}

	// 验证容器是否为Go应用
	// TODO: 实现Go应用检测逻辑

	return nil
}