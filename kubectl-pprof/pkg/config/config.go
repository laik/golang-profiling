package config

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// KubernetesConfig Kubernetes配置
type KubernetesConfig struct {
	Config    *rest.Config
	Clientset kubernetes.Interface
	Namespace string
}

// LoadKubernetesConfig 加载Kubernetes配置
func LoadKubernetesConfig() (*KubernetesConfig, error) {
	// 尝试加载集群内配置
	config, err := rest.InClusterConfig()
	if err != nil {
		// 如果不在集群内，尝试加载kubeconfig
		config, err = loadKubeConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubernetes config: %w", err)
		}
	}

	// 创建客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// 获取当前命名空间
	namespace := getCurrentNamespace()

	return &KubernetesConfig{
		Config:    config,
		Clientset: clientset,
		Namespace: namespace,
	}, nil
}

// loadKubeConfig 加载kubeconfig文件
func loadKubeConfig() (*rest.Config, error) {
	// 获取kubeconfig路径
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig file not found: %s", kubeconfigPath)
	}

	// 加载配置
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// getCurrentNamespace 获取当前命名空间
func getCurrentNamespace() string {
	// 尝试从环境变量获取
	if ns := os.Getenv("KUBECTL_NAMESPACE"); ns != "" {
		return ns
	}

	// 尝试从kubeconfig获取
	if kubeconfigPath := getKubeconfigPath(); kubeconfigPath != "" {
		if ns := getNamespaceFromKubeconfig(kubeconfigPath); ns != "" {
			return ns
		}
	}

	// 默认命名空间
	return "default"
}

// getKubeconfigPath 获取kubeconfig路径
func getKubeconfigPath() string {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}
	return kubeconfigPath
}

// getNamespaceFromKubeconfig 从kubeconfig获取当前命名空间
func getNamespaceFromKubeconfig(kubeconfigPath string) string {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return ""
	}

	if config.CurrentContext == "" {
		return ""
	}

	context, exists := config.Contexts[config.CurrentContext]
	if !exists {
		return ""
	}

	return context.Namespace
}

// ValidateAccess 验证访问权限
func (k *KubernetesConfig) ValidateAccess(namespace string) error {
	// TODO: 实现权限验证逻辑
	// 检查是否有创建Job的权限
	// 检查是否有访问Pod的权限
	return nil
}