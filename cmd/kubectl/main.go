package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
	"mini-k8s/pkg/types"
)

const apiServerURL = "http://localhost:8080"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "apply":
		if len(os.Args) < 4 || os.Args[2] != "-f" {
			fmt.Println("Usage: kubectl apply -f <filename>")
			os.Exit(1)
		}
		apply(os.Args[3])
	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kubectl get pods")
			os.Exit(1)
		}
		get(os.Args[2])
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Usage: kubectl delete pod <name>")
			os.Exit(1)
		}
		deleteResource(os.Args[2], os.Args[3])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: kubectl <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  apply -f <filename>     Apply a configuration to a resource")
	fmt.Println("  get pods                List all pods")
	fmt.Println("  delete pod <name>       Delete a pod")
}

func apply(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	var pod types.Pod
	if err := yaml.Unmarshal(data, &pod); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// 验证
	if pod.Kind != "Pod" {
		fmt.Printf("Error: unsupported kind '%s', only 'Pod' is supported\n", pod.Kind)
		os.Exit(1)
	}
	if pod.Metadata.Name == "" {
		fmt.Println("Error: pod name is required")
		os.Exit(1)
	}

	// 发送到 API Server
	jsonData, _ := json.Marshal(&pod)
	resp, err := http.Post(apiServerURL+"/api/v1/pods", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error connecting to API Server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("pod/%s created\n", pod.Metadata.Name)
	} else if resp.StatusCode == http.StatusOK {
		fmt.Printf("pod/%s configured\n", pod.Metadata.Name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}
}

func get(resource string) {
	if resource != "pods" && resource != "pod" {
		fmt.Printf("Error: unsupported resource '%s', only 'pods' is supported\n", resource)
		os.Exit(1)
	}

	resp, err := http.Get(apiServerURL + "/api/v1/pods")
	if err != nil {
		fmt.Printf("Error connecting to API Server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: unexpected status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var list types.PodList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		fmt.Printf("Error decoding response: %v\n", err)
		os.Exit(1)
	}

	// 打印表格
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tREADY\tSTATUS\tRESTARTS\tAGE")

	for _, pod := range list.Items {
		name := pod.Metadata.Name
		status := pod.Status.Phase
		if status == "" {
			status = "Pending"
		}

		ready := "0/1"
		restarts := 0

		if len(pod.Status.ContainerStatuses) > 0 {
			containerStatus := pod.Status.ContainerStatuses[0]
			if containerStatus.Ready {
				ready = "1/1"
			}
		}

		age := "<unknown>"
		if pod.Status.StartTime != nil {
			age = "0s" // 简化，不计算真实时间差
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", name, ready, status, restarts, age)
	}

	w.Flush()
}

func deleteResource(resource, name string) {
	if resource != "pod" && resource != "pods" {
		fmt.Printf("Error: unsupported resource '%s'\n", resource)
		os.Exit(1)
	}

	url := fmt.Sprintf("%s/api/v1/pods/%s", apiServerURL, name)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error connecting to API Server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		fmt.Printf("pod \"%s\" deleted\n", name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}
}
