# mini-k8s

一个单机版的简化 Kubernetes 实现，用于学习 K8s 核心工作原理。

支持通过 `kubectl apply -f pod.yaml` 创建 Pod，并在本机通过 Docker 运行容器。

---

## 项目结构

```
mini-k8s/
├── cmd/
│   ├── apiserver/   # API Server，暴露 REST 接口，持久化 Pod 数据
│   ├── kubelet/     # Kubelet，轮询 API Server，驱动 Docker 创建/删除容器
│   └── kubectl/     # 简化版 kubectl，解析 YAML 并调用 API
├── pkg/
│   ├── types/       # Pod 数据结构定义
│   └── storage/     # BoltDB 存储封装
├── test/
│   └── integration/ # API Server 集成测试
├── examples/        # 示例 Pod YAML 文件
└── docs/design.md   # 详细设计方案文档
```

---

## 依赖要求

- Go 1.21+
- Docker Desktop（或 Docker Engine）

---

## 运行步骤

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动 API Server（终端 1）

```bash
go run cmd/apiserver/main.go
```

> 默认监听 `http://localhost:8080`

### 3. 启动 Kubelet（终端 2）

```bash
go run cmd/kubelet/main.go
```

> Kubelet 每 5 秒轮询一次 API Server，自动同步容器状态

### 4. 操作 Pod（终端 3）

```bash
# 创建 Pod
go run cmd/kubectl/main.go apply -f examples/nginx-pod.yaml

# 查看 Pod 列表
go run cmd/kubectl/main.go get pods

# 删除 Pod
go run cmd/kubectl/main.go delete pod nginx
```

---

## 示例输出

```
# 创建
pod/nginx created

# 查看
NAME    READY   STATUS    RESTARTS   AGE
nginx   1/1     Running   0          0s

# 删除
pod "nginx" deleted
```

---

## 运行测试

```bash
# 单元测试
go test ./pkg/...

# 集成测试
go test ./test/integration/
```

---

## 技术选型

| 组件 | 选择 |
|------|------|
| 语言 | Go |
| 存储 | BoltDB（嵌入式，无需额外进程） |
| 容器运行时 | Docker CLI |
| API 协议 | HTTP REST |

详细设计见 [docs/design.md](docs/design.md)
