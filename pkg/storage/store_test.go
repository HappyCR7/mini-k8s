package storage

import (
	"os"
	"testing"

	"mini-k8s/pkg/types"
)

// newTestStore 创建一个临时数据库，测试完自动清理
func newTestStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "mini-k8s-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	f.Close()

	store, err := NewStore(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(f.Name())
	})
	return store
}

func samplePod(name, namespace string) *types.Pod {
	return &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.Metadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{Name: name, Image: "nginx:latest"},
			},
		},
	}
}

// --- Put & Get ---

func TestStore_PutAndGet(t *testing.T) {
	store := newTestStore(t)
	pod := samplePod("nginx", "default")

	if err := store.Put("default/nginx", pod); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	var got types.Pod
	if err := store.Get("default/nginx", &got); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Metadata.Name != pod.Metadata.Name {
		t.Errorf("Name = %q, want %q", got.Metadata.Name, pod.Metadata.Name)
	}
	if got.Spec.Containers[0].Image != "nginx:latest" {
		t.Errorf("Image = %q, want %q", got.Spec.Containers[0].Image, "nginx:latest")
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	store := newTestStore(t)

	var pod types.Pod
	err := store.Get("default/nonexistent", &pod)
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

// --- Put 覆盖更新 ---

func TestStore_Put_Overwrite(t *testing.T) {
	store := newTestStore(t)
	pod := samplePod("nginx", "default")

	store.Put("default/nginx", pod)

	// 更新 image
	pod.Spec.Containers[0].Image = "nginx:1.25"
	if err := store.Put("default/nginx", pod); err != nil {
		t.Fatalf("Put (overwrite) failed: %v", err)
	}

	var got types.Pod
	store.Get("default/nginx", &got)
	if got.Spec.Containers[0].Image != "nginx:1.25" {
		t.Errorf("Image after overwrite = %q, want %q", got.Spec.Containers[0].Image, "nginx:1.25")
	}
}

// --- Delete ---

func TestStore_Delete(t *testing.T) {
	store := newTestStore(t)
	pod := samplePod("nginx", "default")
	store.Put("default/nginx", pod)

	if err := store.Delete("default/nginx"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var got types.Pod
	if err := store.Get("default/nginx", &got); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestStore_Delete_NonExistentKey(t *testing.T) {
	store := newTestStore(t)
	// BoltDB 删除不存在的 key 不报错，这里验证该行为
	if err := store.Delete("default/ghost"); err != nil {
		t.Errorf("Delete non-existent key should not error, got: %v", err)
	}
}

// --- List ---

func TestStore_List_Empty(t *testing.T) {
	store := newTestStore(t)

	var count int
	err := store.List(
		func() interface{} { return &types.Pod{} },
		func(obj interface{}) { count++ },
	)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestStore_List_MultiplePods(t *testing.T) {
	store := newTestStore(t)

	pods := []*types.Pod{
		samplePod("nginx", "default"),
		samplePod("redis", "default"),
		samplePod("mysql", "production"),
	}
	for _, p := range pods {
		store.Put(p.GetKey(), p)
	}

	var result []types.Pod
	err := store.List(
		func() interface{} { return &types.Pod{} },
		func(obj interface{}) {
			result = append(result, *(obj.(*types.Pod)))
		},
	)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("List count = %d, want 3", len(result))
	}
}

func TestStore_List_AfterDelete(t *testing.T) {
	store := newTestStore(t)

	store.Put("default/nginx", samplePod("nginx", "default"))
	store.Put("default/redis", samplePod("redis", "default"))
	store.Delete("default/nginx")

	var result []types.Pod
	store.List(
		func() interface{} { return &types.Pod{} },
		func(obj interface{}) {
			result = append(result, *(obj.(*types.Pod)))
		},
	)
	if len(result) != 1 {
		t.Errorf("List count after delete = %d, want 1", len(result))
	}
	if result[0].Metadata.Name != "redis" {
		t.Errorf("remaining pod = %q, want %q", result[0].Metadata.Name, "redis")
	}
}

// --- Close ---

func TestStore_Close(t *testing.T) {
	f, _ := os.CreateTemp("", "mini-k8s-close-*.db")
	f.Close()
	defer os.Remove(f.Name())

	store, err := NewStore(f.Name())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
