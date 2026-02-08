package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"mini-k8s/pkg/types"
)

type Kubelet struct {
	apiServerURL string
	nodeName     string
	knownPods    map[string]*types.Pod
}

func main() {
	k := &Kubelet{
		apiServerURL: "http://localhost:8080",
		nodeName:     "minikube",
		knownPods:    make(map[string]*types.Pod),
	}

	fmt.Printf("[Kubelet] Starting on node: %s\n", k.nodeName)
	fmt.Printf("[Kubelet] Connecting to API Server: %s\n", k.apiServerURL)

	// 每 5 秒同步一次
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	k.syncLoop()

	for range ticker.C {
		k.syncLoop()
	}
}

func (k *Kubelet) syncLoop() {
	pods, err := k.getPodsFromAPIServer()
	if err != nil {
		fmt.Printf("[Kubelet] Error getting pods: %v\n", err)
		return
	}

	currentPods := make(map[string]bool)

	for _, pod := range pods {
		key := pod.GetKey()
		currentPods[key] = true

		if _, exists := k.knownPods[key]; !exists {
			// 新 Pod，创建容器
			fmt.Printf("[Kubelet] Detected new pod: %s\n", key)
			k.createContainer(&pod)
		}
		k.knownPods[key] = &pod
	}

	// 检查已删除的 Pod
	for key, pod := range k.knownPods {
		if !currentPods[key] {
			fmt.Printf("[Kubelet] Pod deleted: %s\n", key)
			k.deleteContainer(pod)
			delete(k.knownPods, key)
		}
	}
}

func (k *Kubelet) getPodsFromAPIServer() ([]types.Pod, error) {
	resp, err := http.Get(k.apiServerURL + "/api/v1/pods")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var list types.PodList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (k *Kubelet) createContainer(pod *types.Pod) {
	if len(pod.Spec.Containers) == 0 {
		fmt.Printf("[Kubelet] Pod %s has no containers\n", pod.Metadata.Name)
		return
	}

	container := pod.Spec.Containers[0] // 简化：只处理第一个容器
	containerName := fmt.Sprintf("%s_%s", pod.Metadata.Name, container.Name)

	// 构建 docker run 命令
	args := []string{"run", "-d", "--name", containerName}

	// 添加端口映射
	for _, port := range container.Ports {
		args = append(args, "-p", fmt.Sprintf("0:%d", port.ContainerPort))
	}

	// 添加环境变量
	for _, env := range container.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	args = append(args, container.Image)

	fmt.Printf("[Kubelet] Creating container: docker %s\n", strings.Join(args, " "))

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[Kubelet] Failed to create container: %v, output: %s\n", err, output)
		k.updatePodStatus(pod, "Failed", "")
		return
	}

	containerID := strings.TrimSpace(string(output))
	fmt.Printf("[Kubelet] Container created: %s\n", containerID[:12])

	k.updatePodStatus(pod, "Running", containerID[:12])
	k.updateContainerStatus(pod, container.Name, containerID[:12], "Running", true)
}

func (k *Kubelet) deleteContainer(pod *types.Pod) {
	if len(pod.Spec.Containers) == 0 {
		return
	}

	for _, container := range pod.Spec.Containers {
		containerName := fmt.Sprintf("%s_%s", pod.Metadata.Name, container.Name)
		fmt.Printf("[Kubelet] Deleting container: %s\n", containerName)

		cmd := exec.Command("docker", "rm", "-f", containerName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("[Kubelet] Failed to delete container: %v, output: %s\n", err, output)
		} else {
			fmt.Printf("[Kubelet] Container deleted: %s\n", containerName)
		}
	}
}

func (k *Kubelet) updatePodStatus(pod *types.Pod, phase, containerID string) {
	status := types.PodStatus{
		Phase: phase,
		PodIP: pod.Status.PodIP,
	}

	if phase == "Running" {
		status.PodIP = k.getContainerIP(pod)
		now := time.Now()
		status.StartTime = &now
	}

	data, _ := json.Marshal(status)
	url := fmt.Sprintf("%s/api/v1/pods/%s/status?namespace=%s", 
		k.apiServerURL, pod.Metadata.Name, pod.Metadata.Namespace)
	
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[Kubelet] Failed to update pod status: %v\n", err)
		return
	}
	defer resp.Body.Close()
}

func (k *Kubelet) updateContainerStatus(pod *types.Pod, containerName, containerID, state string, ready bool) {
	status := types.PodStatus{
		Phase: "Running",
		ContainerStatuses: []types.ContainerStatus{
			{
				Name:        containerName,
				ContainerID: containerID,
				State:       state,
				Ready:       ready,
			},
		},
	}

	data, _ := json.Marshal(status)
	url := fmt.Sprintf("%s/api/v1/pods/%s/status?namespace=%s",
		k.apiServerURL, pod.Metadata.Name, pod.Metadata.Namespace)

	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[Kubelet] Failed to update container status: %v\n", err)
		return
	}
	defer resp.Body.Close()
}

func (k *Kubelet) getContainerIP(pod *types.Pod) string {
	if len(pod.Spec.Containers) == 0 {
		return ""
	}
	containerName := fmt.Sprintf("%s_%s", pod.Metadata.Name, pod.Spec.Containers[0].Name)
	inspectCmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName)
	ipOutput, _ := inspectCmd.Output()
	return strings.TrimSpace(string(ipOutput))
}

func init() {
	// 检查 docker 是否可用
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		log.Fatal("Docker is not available. Please install and start Docker.")
	}
}
