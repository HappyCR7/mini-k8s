package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"mini-k8s/pkg/storage"
	"mini-k8s/pkg/types"
)

type Server struct {
	store *storage.Store
}

func main() {
	store, err := storage.NewStore("mini-k8s.db")
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	s := &Server{store: store}

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/pods", s.createPod).Methods("POST")
	r.HandleFunc("/api/v1/pods", s.listPods).Methods("GET")
	r.HandleFunc("/api/v1/pods/{name}", s.getPod).Methods("GET")
	r.HandleFunc("/api/v1/pods/{name}", s.deletePod).Methods("DELETE")
	r.HandleFunc("/api/v1/pods/{name}/status", s.updatePodStatus).Methods("PUT")

	addr := ":8080"
	fmt.Printf("API Server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (s *Server) createPod(w http.ResponseWriter, r *http.Request) {
	var pod types.Pod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 设置默认值
	if pod.APIVersion == "" {
		pod.APIVersion = "v1"
	}
	if pod.Kind == "" {
		pod.Kind = "Pod"
	}
	if pod.Metadata.Namespace == "" {
		pod.Metadata.Namespace = "default"
	}

	// 初始状态
	now := time.Now()
	pod.Status.Phase = "Pending"
	pod.Status.StartTime = &now

	key := pod.GetKey()
	if err := s.store.Put(key, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[API Server] Created pod: %s\n", key)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&pod)
}

func (s *Server) listPods(w http.ResponseWriter, r *http.Request) {
	var pods []types.Pod
	err := s.store.List(
		func() interface{} { return &types.Pod{} },
		func(obj interface{}) {
			pods = append(pods, *(obj.(*types.Pod)))
		},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	list := types.PodList{
		APIVersion: "v1",
		Kind:       "PodList",
		Items:      pods,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&list)
}

func (s *Server) getPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + name

	var pod types.Pod
	if err := s.store.Get(key, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&pod)
}

func (s *Server) deletePod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + name

	if err := s.store.Delete(key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[API Server] Deleted pod: %s\n", key)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updatePodStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "default"
	}
	key := ns + "/" + name

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
	if err := s.store.Put(key, &pod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&pod)
}

func extractKey(name, defaultNs string) string {
	if strings.Contains(name, "/") {
		return name
	}
	return defaultNs + "/" + name
}
