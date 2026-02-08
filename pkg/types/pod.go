package types

import (
	"time"
)

// Pod 是 Kubernetes Pod 的简化版
type Pod struct {
	APIVersion string    `json:"apiVersion" yaml:"apiVersion"`
	Kind       string    `json:"kind" yaml:"kind"`
	Metadata   Metadata  `json:"metadata" yaml:"metadata"`
	Spec       PodSpec   `json:"spec" yaml:"spec"`
	Status     PodStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// Metadata 元数据
type Metadata struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// PodSpec Pod 规格
type PodSpec struct {
	Containers []Container `json:"containers" yaml:"containers"`
}

// Container 容器定义
type Container struct {
	Name  string   `json:"name" yaml:"name"`
	Image string   `json:"image" yaml:"image"`
	Ports []Port   `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env   []EnvVar `json:"env,omitempty" yaml:"env,omitempty"`
}

// Port 端口映射
type Port struct {
	ContainerPort int    `json:"containerPort" yaml:"containerPort"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
}

// EnvVar 环境变量
type EnvVar struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// PodStatus Pod 状态
type PodStatus struct {
	Phase             string            `json:"phase,omitempty" yaml:"phase,omitempty"`
	PodIP             string            `json:"podIP,omitempty" yaml:"podIP,omitempty"`
	StartTime         *time.Time        `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty" yaml:"containerStatuses,omitempty"`
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	Name        string `json:"name" yaml:"name"`
	ContainerID string `json:"containerID,omitempty" yaml:"containerID,omitempty"`
	Image       string `json:"image" yaml:"image"`
	State       string `json:"state,omitempty" yaml:"state,omitempty"`
	Ready       bool   `json:"ready" yaml:"ready"`
}

// PodList Pod 列表
type PodList struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Items      []Pod  `json:"items" yaml:"items"`
}

// GetKey 获取 Pod 的存储键
func (p *Pod) GetKey() string {
	ns := p.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}
	return ns + "/" + p.Metadata.Name
}
