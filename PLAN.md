# mini-k8s 实现计划

## 目标
实现一个**单机版 mini-k8s**，支持 `kubectl apply -f pod.yaml` 创建 Pod，并在本机运行容器。

---

## 技术选型

| 组件 | 选择 | 原因 |
|------|------|------|
| **语言** | Go | 生态好，containerd 原生支持 |
| **存储** | BoltDB | 嵌入式，无需额外进程 |
| **容器运行时** | Docker CLI | 简化实现，用户机器通常都有 |
| **API 协议** | HTTP REST | 简单，易调试 |

---

## 项目结构

```
mini-k8s/
├── cmd/
│   ├── apiserver/      # API Server（存储 Pod，暴露 REST API）
│   ├── kubelet/        # Kubelet（监听 Pod，创建容器）
│   └── kubectl/        # 简化版 kubectl（解析 YAML，调用 API）
├── pkg/
│   ├── types/          # Pod 结构体定义
│   ├── storage/        # BoltDB 封装
│   └── client/         # HTTP 客户端封装
├── go.mod
└── examples/
    └── nginx-pod.yaml  # 测试用的 YAML
```

---

## 实现步骤

### Phase 1：基础类型和存储（约 30 分钟）
- [ ] 定义 `Pod`、`Container` 等结构体
- [ ] 实现 BoltDB 存储层（CRUD 操作）

### Phase 2：API Server（约 40 分钟）
- [ ] 启动 HTTP 服务（端口 8080）
- [ ] 实现以下接口：
  - `POST /api/v1/pods` - 创建 Pod
  - `GET /api/v1/pods` - 列出所有 Pod
  - `GET /api/v1/pods/{name}` - 获取单个 Pod
  - `DELETE /api/v1/pods/{name}` - 删除 Pod
- [ ] 数据持久化到 BoltDB

### Phase 3：Kubelet（约 40 分钟）
- [ ] 定时轮询 API Server 获取 Pod 列表
- [ ] 对比本地容器状态，执行：
  - 新 Pod → `docker run` 创建容器
  - 已删除 Pod → `docker rm -f` 删除容器
- [ ] 更新 Pod 状态（Running/Pending）回 API Server

### Phase 4：kubectl（约 30 分钟）
- [ ] 解析 YAML 文件
- [ ] 调用 API Server 创建 Pod
- [ ] 支持 `kubectl apply -f pod.yaml`

### Phase 5：测试验证（约 20 分钟）
- [ ] 启动 API Server
- [ ] 启动 Kubelet
- [ ] 执行 `kubectl apply -f examples/nginx-pod.yaml`
- [ ] 验证 `docker ps` 看到容器运行

---

## 简化设计

| 功能 | 完整版 | 本次实现（简化） |
|------|--------|----------------|
| 容器运行时 | containerd SDK | Docker CLI 命令 |
| 网络 | CNI 插件 | Docker 默认网络 |
| 调度器 | 复杂算法 | 直接本机运行 |
| 多容器 Pod | 支持 | 先支持单容器 |
| 存储 | 多种 Volume | 先不做 Volume |
| 状态更新 | Watch 机制 | 轮询（每 5 秒） |

---

## 依赖要求

- **Go** (1.21+)
- **Docker Desktop**（或 Docker Engine）

---

## 验证方式

```bash
# 1. 启动 API Server
go run cmd/apiserver/main.go

# 2. 另一个终端启动 Kubelet
go run cmd/kubelet/main.go

# 3. 创建 Pod
go run cmd/kubectl/main.go apply -f examples/nginx-pod.yaml

# 4. 查看
curl http://localhost:8080/api/v1/pods
docker ps
```

---

## 状态

- 计划创建时间: 2026-02-06
- 当前阶段: **已完成** ✓

### 完成清单
- [x] Phase 1: 基础类型和存储
- [x] Phase 2: API Server
- [x] Phase 3: Kubelet
- [x] Phase 4: kubectl
- [x] Phase 5: 测试文件

### 运行步骤

#### 方式一：直接运行（开发调试用）
```bash
# 1. 下载依赖
cd mini-k8s
"C:\Program Files\Go\bin\go.exe" mod tidy

# 2. 启动 API Server（终端1）
"C:\Program Files\Go\bin\go.exe" run cmd/apiserver/main.go

# 3. 启动 Kubelet（终端2）
"C:\Program Files\Go\bin\go.exe" run cmd/kubelet/main.go

# 4. 创建 Pod（终端3）
"C:\Program Files\Go\bin\go.exe" run cmd/kubectl/main.go apply -f examples/nginx-pod.yaml

# 5. 查看
"C:\Program Files\Go\bin\go.exe" run cmd/kubectl/main.go get pods
docker ps
```

#### 方式二：编译后运行（推荐）
```bash
# 1. 编译（已编译完成）
"C:\Program Files\Go\bin\go.exe" build ./cmd/apiserver
"C:\Program Files\Go\bin\go.exe" build ./cmd/kubelet
"C:\Program Files\Go\bin\go.exe" build ./cmd/kubectl

# 2. 启动 API Server（终端1）
.\apiserver.exe

# 3. 启动 Kubelet（终端2）
.\kubelet.exe

# 4. 创建 Pod（终端3）
.\kubectl.exe apply -f examples/nginx-pod.yaml
# 输出: pod/nginx created

# 5. 查看 Pod 列表
.\kubectl.exe get pods
# 输出:
# NAME    READY   STATUS    RESTARTS   AGE
# nginx   1/1     Running   0          0s

# 6. 验证 Docker 容器
.\kubectl.exe apply -f examples/redis-pod.yaml
docker ps
# 应该看到 nginx_nginx 和 redis_redis 容器

# 7. 删除 Pod
.\kubectl.exe delete pod nginx
# 输出: pod "nginx" deleted
```
