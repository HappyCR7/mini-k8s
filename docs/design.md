# mini-k8s 设计方案

## 1. 项目概览

mini-k8s 是一个单机版的简化 Kubernetes 实现，目标是用最少的代码还原 Kubernetes 的核心工作流：

> 用户提交 YAML → API Server 持久化 → Kubelet 轮询执行 → Docker 容器运行

本项目用于学习 Kubernetes 内部机制，不用于生产环境。

---

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                        用户 / 操作员                          │
└───────────────────────────┬─────────────────────────────────┘
                            │ kubectl apply -f pod.yaml
                            │ kubectl get pods
                            │ kubectl delete pod <name>
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    kubectl（CLI 客户端）                       │
│  • 解析 YAML 文件                                             │
│  • 转换为 JSON 调用 REST API                                  │
│  • 格式化输出结果                                             │
└───────────────────────────┬─────────────────────────────────┘
                            │ HTTP REST（:8080）
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Server（核心）                          │
│  • 暴露 REST API                                              │
│  • 校验请求、设置默认值                                        │
│  • 读写 BoltDB 持久化存储                                     │
│                                                             │
│  存储层：BoltDB（mini-k8s.db）                                │
└───────────────────────────┬─────────────────────────────────┘
                            │ HTTP 轮询（每 5 秒）
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Kubelet（节点代理）                       │
│  • 定时拉取 API Server 的 Pod 列表                            │
│  • 对比本地已知状态，计算 diff                                 │
│  • 新增 Pod → docker run                                     │
│  • 删除 Pod → docker rm -f                                   │
│  • 更新 Pod 状态回写 API Server                               │
└───────────────────────────┬─────────────────────────────────┘
                            │ docker CLI 调用
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Docker Engine                           │
│  • 管理容器生命周期                                            │
│  • 提供网络 / 端口映射                                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. 组件职责

### 3.1 kubectl（`cmd/kubectl`）

| 职责 | 说明 |
|------|------|
| YAML 解析 | 使用 `gopkg.in/yaml.v3` 将 pod.yaml 反序列化为 `types.Pod` |
| 参数解析 | 支持 `apply -f`、`get pods`、`delete pod <name>` 三条命令 |
| API 调用 | 将结构体序列化为 JSON，通过标准库 `net/http` 调用 API Server |
| 输出格式化 | 使用 `text/tabwriter` 对齐输出表格 |

kubectl **不持有任何状态**，每次运行都是无状态的 HTTP 调用。

---

### 3.2 API Server（`cmd/apiserver`）

| 职责 | 说明 |
|------|------|
| 路由 | 使用 `gorilla/mux` 注册 REST 路由 |
| 数据校验 | 检查 Kind、Name 等必填字段，填充默认 Namespace、初始状态 |
| 持久化 | 通过 `pkg/storage.Store` 读写 BoltDB |
| 状态更新 | 提供专用的 `/status` 子资源接口供 Kubelet 回写 |

API Server 是**唯一有权读写存储**的组件，其他组件必须通过 HTTP 与它交互。

---

### 3.3 Kubelet（`cmd/kubelet`）

| 职责 | 说明 |
|------|------|
| 同步循环 | 每 5 秒执行一次 `syncLoop` |
| Diff 计算 | 将 API Server 返回的 Pod 列表与本地 `knownPods` map 对比 |
| 创建容器 | 调用 `docker run -d` 启动容器，处理端口映射和环境变量 |
| 删除容器 | 调用 `docker rm -f` 强制删除容器 |
| 状态上报 | 容器启动后，通过 `PUT /pods/{name}/status` 更新为 Running |
| IP 获取 | 通过 `docker inspect` 获取容器 IP 写入 PodIP |

Kubelet **不直接操作存储**，只能通过 API Server 获取期望状态和上报实际状态。

---

### 3.4 存储层（`pkg/storage`）

基于 [BoltDB](https://github.com/etcd-io/bbolt)（bbolt）的简单 KV 封装。

| 方法 | 说明 |
|------|------|
| `Put(key, obj)` | 将任意对象 JSON 序列化后写入 `pods` bucket |
| `Get(key, obj)` | 读取并 JSON 反序列化到目标对象 |
| `Delete(key)` | 删除指定 key |
| `List(factory, callback)` | 遍历 bucket 中所有条目，逐个反序列化并回调 |

Key 格式统一为 `<namespace>/<name>`，例如 `default/nginx`。

---

### 3.5 数据类型（`pkg/types`）

```
Pod
├── APIVersion  string
├── Kind        string
├── Metadata
│   ├── Name       string
│   ├── Namespace  string
│   └── Labels     map[string]string
├── Spec
│   └── Containers []Container
│       ├── Name   string
│       ├── Image  string
│       ├── Ports  []Port    (ContainerPort, Name)
│       └── Env    []EnvVar  (Name, Value)
└── Status
    ├── Phase             string  (Pending / Running / Failed)
    ├── PodIP             string
    ├── StartTime         *time.Time
    └── ContainerStatuses []ContainerStatus
        ├── Name        string
        ├── ContainerID string
        ├── Image       string
        ├── State       string
        └── Ready       bool
```

---

## 4. REST API 规范

Base URL: `http://localhost:8080`

| 方法 | 路径 | 说明 | 请求体 | 成功响应 |
|------|------|------|--------|---------|
| `POST` | `/api/v1/pods` | 创建 Pod | `Pod` JSON | `201 Created`，返回 Pod |
| `GET` | `/api/v1/pods` | 列出所有 Pod | 无 | `200 OK`，返回 `PodList` |
| `GET` | `/api/v1/pods/{name}` | 获取单个 Pod | 无 | `200 OK`，返回 Pod |
| `DELETE` | `/api/v1/pods/{name}` | 删除 Pod | 无 | `204 No Content` |
| `PUT` | `/api/v1/pods/{name}/status` | 更新 Pod 状态 | `PodStatus` JSON | `200 OK`，返回 Pod |

**查询参数**：`GET/DELETE /api/v1/pods/{name}` 支持 `?namespace=<ns>`，默认为 `default`。

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| `400 Bad Request` | 请求体解析失败 |
| `404 Not Found` | Pod 不存在 |
| `500 Internal Server Error` | 存储层错误 |

---

## 5. 核心数据流

### 5.1 创建 Pod（`kubectl apply -f pod.yaml`）

```
kubectl                  API Server               BoltDB
   │                         │                       │
   │── 解析 YAML ──▶          │                       │
   │── POST /api/v1/pods ──▶  │                       │
   │                         │── 填充默认值            │
   │                         │   Namespace = default  │
   │                         │   Status = Pending     │
   │                         │── Put("default/nginx") ──▶ │
   │                         │◀─ ok ─────────────────────│
   │◀── 201 + Pod JSON ───── │                       │
   │
   │（5 秒后）
   │
Kubelet                  API Server               Docker
   │── GET /api/v1/pods ──▶  │                       │
   │◀── PodList ─────────── │                       │
   │── 发现新 Pod nginx       │                       │
   │── docker run -d nginx:latest ────────────────▶  │
   │◀── containerID ─────────────────────────────── │
   │── docker inspect ──────────────────────────▶   │
   │◀── IP: 172.17.0.2 ─────────────────────────── │
   │── PUT /api/v1/pods/nginx/status ──▶ │
   │   {phase: Running, podIP: ...}      │
   │◀── 200 + Pod ──────────────────── │
```

### 5.2 删除 Pod（`kubectl delete pod nginx`）

```
kubectl                  API Server               BoltDB
   │── DELETE /api/v1/pods/nginx ──▶ │              │
   │                                 │── Delete ──▶ │
   │◀── 204 ───────────────────────  │              │

（5 秒后）
Kubelet                  API Server               Docker
   │── GET /api/v1/pods ──▶ │                       │
   │◀── PodList（不含nginx）│                       │
   │── 发现 nginx 已消失     │                       │
   │── docker rm -f nginx_nginx ───────────────▶    │
   │◀── ok ──────────────────────────────────────── │
   │── 从 knownPods 删除
```

---

## 6. 与真实 Kubernetes 的差异

| 特性 | 真实 Kubernetes | mini-k8s（本项目）|
|------|----------------|-----------------|
| 容器运行时 | containerd / CRI-O（通过 CRI 接口） | Docker CLI 命令 |
| 存储后端 | etcd（分布式 KV） | BoltDB（单机嵌入式） |
| Kubelet 感知变化 | Watch 机制（长连接推送） | 轮询（每 5 秒 HTTP GET） |
| 调度 | kube-scheduler（多节点打分） | 直接在本机运行 |
| 网络 | CNI 插件（Pod 独立 IP、跨节点互通） | Docker bridge 默认网络 |
| 多容器 Pod | 共享 network namespace | 只处理第一个容器 |
| Volume | 多种 Volume 类型 | 不支持 |
| 命名空间隔离 | 完整 RBAC + Namespace | Namespace 仅用于 key 前缀 |
| 健康检查 | Liveness / Readiness Probe | 不支持 |
| 滚动更新 | Deployment + ReplicaSet | 不支持 |

---

## 7. 目录结构

```
kuberStudy/
├── cmd/
│   ├── apiserver/
│   │   └── main.go          # API Server 入口
│   ├── kubelet/
│   │   └── main.go          # Kubelet 入口
│   └── kubectl/
│       └── main.go          # kubectl CLI 入口
├── pkg/
│   ├── types/
│   │   ├── pod.go           # 数据类型定义
│   │   └── pod_test.go      # 类型单元测试
│   └── storage/
│       ├── store.go         # BoltDB 封装
│       └── store_test.go    # 存储单元测试
├── test/
│   └── integration/
│       └── apiserver_test.go  # API Server 集成测试
├── examples/
│   ├── nginx-pod.yaml       # nginx 示例 Pod
│   └── redis-pod.yaml       # redis 示例 Pod
├── docs/
│   └── design.md            # 本文档
├── .gitignore
├── go.mod
├── go.sum
├── PLAN.md                  # 实现计划
└── AGENTS.md                # AI 协作配置
```

---

## 8. 快速上手

```bash
# 安装依赖
go mod tidy

# 终端 1：启动 API Server
go run cmd/apiserver/main.go

# 终端 2：启动 Kubelet
go run cmd/kubelet/main.go

# 终端 3：操作
go run cmd/kubectl/main.go apply -f examples/nginx-pod.yaml
go run cmd/kubectl/main.go get pods
go run cmd/kubectl/main.go delete pod nginx

# 运行测试
go test ./pkg/...           # 单元测试
go test ./test/integration/ # 集成测试
```

**依赖要求**：Go 1.21+，Docker Desktop（或 Docker Engine）
