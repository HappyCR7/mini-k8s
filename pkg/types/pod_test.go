package types

import (
	"encoding/json"
	"testing"
	"time"
)

// --- GetKey ---

func TestGetKey_WithNamespace(t *testing.T) {
	pod := &Pod{
		Metadata: Metadata{
			Name:      "nginx",
			Namespace: "production",
		},
	}
	want := "production/nginx"
	if got := pod.GetKey(); got != want {
		t.Errorf("GetKey() = %q, want %q", got, want)
	}
}

func TestGetKey_DefaultNamespace(t *testing.T) {
	pod := &Pod{
		Metadata: Metadata{
			Name: "redis",
			// Namespace 留空，期望落到 "default"
		},
	}
	want := "default/redis"
	if got := pod.GetKey(); got != want {
		t.Errorf("GetKey() = %q, want %q", got, want)
	}
}

func TestGetKey_ExplicitDefaultNamespace(t *testing.T) {
	pod := &Pod{
		Metadata: Metadata{
			Name:      "myapp",
			Namespace: "default",
		},
	}
	want := "default/myapp"
	if got := pod.GetKey(); got != want {
		t.Errorf("GetKey() = %q, want %q", got, want)
	}
}

// --- JSON 序列化 / 反序列化 ---

func TestPod_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: Metadata{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
		},
		Spec: PodSpec{
			Containers: []Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []Port{{ContainerPort: 80, Name: "http"}},
					Env:   []EnvVar{{Name: "ENV", Value: "prod"}},
				},
			},
		},
		Status: PodStatus{
			Phase:     "Running",
			PodIP:     "172.17.0.2",
			StartTime: &now,
			ContainerStatuses: []ContainerStatus{
				{
					Name:        "nginx",
					ContainerID: "abc123",
					Image:       "nginx:latest",
					State:       "Running",
					Ready:       true,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Pod
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Metadata.Name != original.Metadata.Name {
		t.Errorf("Name = %q, want %q", decoded.Metadata.Name, original.Metadata.Name)
	}
	if decoded.Spec.Containers[0].Image != original.Spec.Containers[0].Image {
		t.Errorf("Image = %q, want %q", decoded.Spec.Containers[0].Image, original.Spec.Containers[0].Image)
	}
	if decoded.Status.Phase != original.Status.Phase {
		t.Errorf("Phase = %q, want %q", decoded.Status.Phase, original.Status.Phase)
	}
	if !decoded.Status.ContainerStatuses[0].Ready {
		t.Error("ContainerStatus.Ready should be true")
	}
}

func TestPod_JSONOmitEmpty(t *testing.T) {
	pod := &Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   Metadata{Name: "simple"},
	}

	data, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Status 字段全为零值，应被 omitempty 忽略
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if status, ok := m["status"]; ok {
		// status 存在但应为空对象 {}，不应包含 phase 等字段
		statusMap, _ := status.(map[string]interface{})
		if _, hasPhase := statusMap["phase"]; hasPhase {
			t.Error("empty phase should be omitted by omitempty")
		}
	}
}

// --- PodList ---

func TestPodList_JSONRoundTrip(t *testing.T) {
	list := &PodList{
		APIVersion: "v1",
		Kind:       "PodList",
		Items: []Pod{
			{Metadata: Metadata{Name: "pod-1", Namespace: "default"}},
			{Metadata: Metadata{Name: "pod-2", Namespace: "kube-system"}},
		},
	}

	data, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded PodList
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(decoded.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(decoded.Items))
	}
	if decoded.Items[0].Metadata.Name != "pod-1" {
		t.Errorf("Items[0].Name = %q, want %q", decoded.Items[0].Metadata.Name, "pod-1")
	}
}

// --- Container / Port / EnvVar ---

func TestContainer_WithMultiplePorts(t *testing.T) {
	c := Container{
		Name:  "app",
		Image: "myapp:v1",
		Ports: []Port{
			{ContainerPort: 8080, Name: "http"},
			{ContainerPort: 9090, Name: "metrics"},
		},
	}
	if len(c.Ports) != 2 {
		t.Errorf("Ports count = %d, want 2", len(c.Ports))
	}
	if c.Ports[0].ContainerPort != 8080 {
		t.Errorf("Port[0] = %d, want 8080", c.Ports[0].ContainerPort)
	}
}

func TestEnvVar_KeyValue(t *testing.T) {
	env := EnvVar{Name: "DB_HOST", Value: "localhost"}
	if env.Name != "DB_HOST" || env.Value != "localhost" {
		t.Errorf("EnvVar = %+v, unexpected", env)
	}
}
