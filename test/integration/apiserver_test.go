// Package integration 提供针对 API Server 的端到端测试。
// 测试会在内存中启动一个真实的 httptest.Server，使用临时 BoltDB，
// 无需预先运行任何外部进程。
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"mini-k8s/pkg/storage"
	"mini-k8s/pkg/types"
)

// ---- 内嵌 apiserver 逻辑（与 cmd/apiserver/main.go 保持同步）----

type server struct {
	store *storage.Store
}

func newServer(t *testing.T) (*server, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "integration-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	f.Close()

	store, err := storage.NewStore(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatalf("NewStore: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(f.Name())
	}
	return &server{store: store}, cleanup
}

func (s *server) router() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/pods", s.createPod).Methods("POST")
	r.HandleFunc("/api/v1/pods", s.listPods).Methods("GET")
	r.HandleFunc("/api/v1/pods/{name}", s.getPod).Methods("GET")
	r.HandleFunc("/api/v1/pods/{name}", s.deletePod).Methods("DELETE")
	r.HandleFunc("/api/v1/pods/{name}/status", s.updatePodStatus).Methods("PUT")
	return r
}

func (s *server) createPod(w http.ResponseWriter, r *http.Request) {
	var pod types.Pod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if pod.APIVersion == "" {
		pod.APIVersion = "v1"
	}
	if pod.Kind == "" {
		pod.Kind = "Pod"
	}
	if pod.Metadata.Namespace == "" {
		pod.Metadata.Namespace = "default"
	}
	now := time.Now()
	pod.Status.Phase = "Pending"
	pod.Status.StartTime = &now

	if err := s.store.Put(pod.GetKey(), &pod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&pod)
}

func (s *server) listPods(w http.ResponseWriter, r *http.Request) {
	var pods []types.Pod
	s.store.List(
		func() interface{} { return &types.Pod{} },
		func(obj interface{}) { pods = append(pods, *(obj.(*types.Pod))) },
	)
	if pods == nil {
		pods = []types.Pod{}
	}
	list := types.PodList{APIVersion: "v1", Kind: "PodList", Items: pods}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&list)
}

func (s *server) getPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + vars["name"]

	var pod types.Pod
	if err := s.store.Get(key, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&pod)
}

func (s *server) deletePod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + vars["name"]
	if err := s.store.Delete(key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) updatePodStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + vars["name"]

	var status types.PodStatus
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var pod types.Pod
	if err := s.store.Get(key, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	pod.Status = status
	s.store.Put(key, &pod)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&pod)
}

// ---- 测试辅助 ----

func podPayload(name string) []byte {
	pod := types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   types.Metadata{Name: name, Namespace: "default"},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{Name: name, Image: "nginx:latest"},
			},
		},
	}
	data, _ := json.Marshal(pod)
	return data
}

// ---- 测试用例 ----

// TestCreatePod_Success 验证 POST /api/v1/pods 返回 201 并携带正确数据
func TestCreatePod_Success(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))
	if err != nil {
		t.Fatalf("POST /api/v1/pods: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var pod types.Pod
	json.NewDecoder(resp.Body).Decode(&pod)
	if pod.Metadata.Name != "nginx" {
		t.Errorf("Name = %q, want %q", pod.Metadata.Name, "nginx")
	}
	if pod.Status.Phase != "Pending" {
		t.Errorf("Phase = %q, want Pending", pod.Status.Phase)
	}
}

// TestCreatePod_InvalidJSON 验证非法请求体返回 400
func TestCreatePod_InvalidJSON(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBufferString("not-json"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestListPods_Empty 验证无 Pod 时返回空 Items
func TestListPods_Empty(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/v1/pods")
	var list types.PodList
	json.NewDecoder(resp.Body).Decode(&list)

	if len(list.Items) != 0 {
		t.Errorf("Items count = %d, want 0", len(list.Items))
	}
}

// TestListPods_MultiplePods 验证多 Pod 时能全部返回
func TestListPods_MultiplePods(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	for _, name := range []string{"nginx", "redis", "mysql"} {
		http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload(name)))
	}

	resp, _ := http.Get(ts.URL + "/api/v1/pods")
	var list types.PodList
	json.NewDecoder(resp.Body).Decode(&list)

	if len(list.Items) != 3 {
		t.Errorf("Items count = %d, want 3", len(list.Items))
	}
}

// TestGetPod_Success 验证 GET /api/v1/pods/{name} 返回正确 Pod
func TestGetPod_Success(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))

	resp, err := http.Get(ts.URL + "/api/v1/pods/nginx")
	if err != nil {
		t.Fatalf("GET /api/v1/pods/nginx: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var pod types.Pod
	json.NewDecoder(resp.Body).Decode(&pod)
	if pod.Metadata.Name != "nginx" {
		t.Errorf("Name = %q, want nginx", pod.Metadata.Name)
	}
}

// TestGetPod_NotFound 验证不存在的 Pod 返回 404
func TestGetPod_NotFound(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/v1/pods/ghost")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// TestDeletePod_Success 验证 DELETE 后 Pod 不可再 GET
func TestDeletePod_Success(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/pods/nginx", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/pods/nginx: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", resp.StatusCode)
	}

	getResp, _ := http.Get(ts.URL + "/api/v1/pods/nginx")
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("GET after DELETE status = %d, want 404", getResp.StatusCode)
	}
}

// TestDeletePod_ListDecreases 验证删除后 List 数量减少
func TestDeletePod_ListDecreases(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))
	http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("redis")))

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/pods/nginx", nil)
	http.DefaultClient.Do(req)

	resp, _ := http.Get(ts.URL + "/api/v1/pods")
	var list types.PodList
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list.Items) != 1 {
		t.Errorf("Items after delete = %d, want 1", len(list.Items))
	}
}

// TestUpdatePodStatus_Success 验证 PUT /api/v1/pods/{name}/status 能更新状态
func TestUpdatePodStatus_Success(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))

	now := time.Now()
	status := types.PodStatus{
		Phase:     "Running",
		PodIP:     "172.17.0.2",
		StartTime: &now,
		ContainerStatuses: []types.ContainerStatus{
			{Name: "nginx", ContainerID: "abc123", State: "Running", Ready: true},
		},
	}
	data, _ := json.Marshal(status)

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/pods/nginx/status", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("PUT status = %d, want 200", resp.StatusCode)
	}

	var pod types.Pod
	json.NewDecoder(resp.Body).Decode(&pod)
	if pod.Status.Phase != "Running" {
		t.Errorf("Phase = %q, want Running", pod.Status.Phase)
	}
	if pod.Status.PodIP != "172.17.0.2" {
		t.Errorf("PodIP = %q, want 172.17.0.2", pod.Status.PodIP)
	}
}

// TestUpdatePodStatus_NotFound 验证对不存在 Pod 更新状态返回 404
func TestUpdatePodStatus_NotFound(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	status := types.PodStatus{Phase: "Running"}
	data, _ := json.Marshal(status)
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/pods/ghost/status", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// TestFullLifecycle 端到端：创建 → 查询 → 更新状态 → 删除
func TestFullLifecycle(t *testing.T) {
	srv, cleanup := newServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	// 1. 创建
	resp, _ := http.Post(ts.URL+"/api/v1/pods", "application/json", bytes.NewBuffer(podPayload("nginx")))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201", resp.StatusCode)
	}

	// 2. 列表包含该 Pod
	resp, _ = http.Get(ts.URL + "/api/v1/pods")
	var list types.PodList
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list.Items) != 1 {
		t.Fatalf("list count = %d, want 1", len(list.Items))
	}

	// 3. 更新为 Running
	now := time.Now()
	status := types.PodStatus{Phase: "Running", StartTime: &now}
	data, _ := json.Marshal(status)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/pods/nginx/status", ts.URL), bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	var updated types.Pod
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Status.Phase != "Running" {
		t.Errorf("phase after update = %q, want Running", updated.Status.Phase)
	}

	// 4. 删除
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/pods/nginx", nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}

	// 5. 列表为空
	resp, _ = http.Get(ts.URL + "/api/v1/pods")
	var emptyList types.PodList
	json.NewDecoder(resp.Body).Decode(&emptyList)
	if len(emptyList.Items) != 0 {
		t.Errorf("list after delete = %d, want 0", len(emptyList.Items))
	}
}
