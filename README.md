# mini-k8s

一个用 Go 实现的**单机版迷你 Kubernetes**，支持通过 `kubectl apply -f pod.yaml` 创建 Pod，并在本机通过 Docker 运行容器。

## ✨ 功能特性

- **API Server**：提供 REST API，管理 Pod 资源，数据持久化至 BoltDB
- **Kubelet**：定时轮询 API Server，自动创建/删除 Docker 容器，并回报 Pod 状态
- **kubectl**：命令行工具，支持 YAML 文件解析，可创建、查看、删除 Pod

## 🏗️ 架构概览

```
┌─────────────┐    HTTP REST     ┌──────────────────┐
│   kubectl   │ ──────────────→  │   API Server     │
└─────────────┘                  │  (port 8080)     │
                                 │  BoltDB 持久化    │
┌─────────────┐    轮询 (5s)     └──────────────────┘
│   Kubelet   │ ←─────────────→
│             │    更新状态
│  docker run │
│  docker rm  │
└─────────────┘
```

## 📁 目录结构

```
mini-k8s/
├── cmd/
│   ├── apiserver/       # API Server：暴露 REST API，存储 Pod 信息
│   │   └── main.go
│   ├── kubelet/         # Kubelet：监听 API Server，驱动 Docker 容器
│   │   └── main.go
│   └── kubectl/         # kubectl：命令行客户端工具
│       └── main.go
├── pkg/
│   ├── types/           # Pod 相关数据结构定义
│   │   └── pod.go
│   └── storage/         # BoltDB 存储层封装（CRUD）
│       └── store.go
├── examples/
│   ├── nginx-pod.yaml   # Nginx Pod 示例
│   └── redis-pod.yaml   # Redis Pod 示例
├── go.mod
└── go.sum
```

## 🔧 依赖要求

| 依赖 | 版本要求 |
|------|----------|
| [Go](https://go.dev/dl/) | 1.21+ |
| [Docker](https://docs.docker.com/get-docker/) | 任意版本（需已启动） |

## 🚀 快速开始

### 1. 克隆仓库并下载依赖

```bash
git clone https://github.com/HappyCR7/mini-k8s.git
cd mini-k8s
go mod tidy
```

### 2. 编译三个组件

```bash
go build -o apiserver ./cmd/apiserver
go build -o kubelet   ./cmd/kubelet
go build -o kubectl   ./cmd/kubectl
```

> Windows 用户在输出文件名后加 `.exe`，例如 `-o apiserver.exe`。

### 3. 启动 API Server（终端 1）

```bash
./apiserver
# 输出：API Server listening on :8080
```

### 4. 启动 Kubelet（终端 2）

```bash
./kubelet
# 输出：[Kubelet] Starting on node: minikube
#       [Kubelet] Connecting to API Server: http://localhost:8080
```

### 5. 操作 Pod（终端 3）

```bash
# 创建 Nginx Pod
./kubectl apply -f examples/nginx-pod.yaml
# 输出：pod/nginx created

# 查看 Pod 列表
./kubectl get pods
# 输出：
# NAME    READY   STATUS    RESTARTS   AGE
# nginx   1/1     Running   0          0s

# 创建 Redis Pod（带环境变量）
./kubectl apply -f examples/redis-pod.yaml

# 验证 Docker 容器已启动
docker ps

# 删除 Pod
./kubectl delete pod nginx
# 输出：pod "nginx" deleted
```

## 📦 示例 YAML

### Nginx Pod

```yaml
# examples/nginx-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: web
spec:
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
      name: http
```

### Redis Pod（带环境变量）

```yaml
# examples/redis-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: redis
  labels:
    app: cache
spec:
  containers:
  - name: redis
    image: redis:alpine
    ports:
    - containerPort: 6379
      name: redis
    env:
    - name: REDIS_PASSWORD
      value: "secret123"
```

## 🔌 API 接口

API Server 监听在 `http://localhost:8080`，提供以下 REST 接口：

| 方法 | 路径 | 描述 |
|------|------|------|
| `POST` | `/api/v1/pods` | 创建 Pod |
| `GET` | `/api/v1/pods` | 列出所有 Pod |
| `GET` | `/api/v1/pods/{name}` | 获取指定 Pod |
| `DELETE` | `/api/v1/pods/{name}` | 删除指定 Pod |
| `PUT` | `/api/v1/pods/{name}/status` | 更新 Pod 状态 |

> 可通过 `?namespace=<ns>` 查询参数指定命名空间，默认为 `default`。

### 示例：通过 curl 直接调用 API

```bash
# 查看所有 Pod
curl http://localhost:8080/api/v1/pods

# 查看指定 Pod
curl http://localhost:8080/api/v1/pods/nginx

# 删除 Pod
curl -X DELETE http://localhost:8080/api/v1/pods/nginx
```

## ⚙️ kubectl 命令

| 命令 | 描述 |
|------|------|
| `kubectl apply -f <file>` | 根据 YAML 文件创建 Pod |
| `kubectl get pods` | 列出所有 Pod |
| `kubectl delete pod <name>` | 删除指定 Pod |

## 🛠️ 技术选型

| 组件 | 选择 | 说明 |
|------|------|------|
| 语言 | Go 1.21 | 性能好，并发模型简洁 |
| HTTP 路由 | [gorilla/mux](https://github.com/gorilla/mux) | 支持路径参数解析 |
| 持久化存储 | [BoltDB (bbolt)](https://github.com/etcd-io/bbolt) | 嵌入式 KV 数据库，无需额外进程 |
| YAML 解析 | [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | 解析 Pod 配置文件 |
| 容器运行时 | Docker CLI | 简化实现，调用本地 Docker 命令 |

## 🔄 工作流程

```
kubectl apply -f pod.yaml
       │
       ▼
  解析 YAML 文件
       │
       ▼
  POST /api/v1/pods  ──→  API Server 存储 Pod（状态: Pending）
                                   │
                           BoltDB 持久化
                                   │
                    Kubelet 每 5 秒轮询 GET /api/v1/pods
                                   │
                    发现新 Pod → docker run 创建容器
                                   │
                    PUT /api/v1/pods/{name}/status（状态: Running）
```

## 📝 简化说明

与完整 Kubernetes 相比，本项目做了如下简化：

| 功能 | 完整版 K8s | mini-k8s |
|------|-----------|----------|
| 容器运行时 | containerd | Docker CLI |
| 网络 | CNI 插件 | Docker 默认网络 |
| 调度器 | 复杂调度算法 | 直接在本机运行 |
| 多容器 Pod | 完整支持 | 仅处理第一个容器 |
| 存储卷 | 多种 Volume 类型 | 暂不支持 |
| 状态同步 | Watch 机制 | 5 秒轮询 |
| 多节点 | 支持 | 单机运行 |

## 📄 License

MIT
