# crossPlatformContainerMonitor 项目说明

## 技术架构

本项目是一个轻量级的跨设备 Docker 容器监控面板，整体采用 `中心服务端 + 边缘 Agent + Web 控制台` 的三层结构。

### 1. 中心服务端

目录：`mini-portainer-server`

职责：

- 接收各节点上报的系统状态和容器信息
- 缓存前端下发的容器控制命令
- 接收 Agent 回传的容器日志
- 提供 Web 控制台页面和 HTTP API

服务端监听 `9000` 端口，内部以 Go 内存结构保存节点状态、命令队列和日志缓存，不依赖数据库。

### 2. 边缘 Agent

目录：

- `Mini Portainer -arm`
- `Mini Portainer -×86`

职责：

- 周期性采集本机 CPU 温度、内存信息
- 读取本机 Docker 容器列表和运行状态
- 将采集结果上报给中心服务端
- 从服务端领取待执行命令并在本机执行
- 按需抓取容器日志并回传

两个 Agent 的核心逻辑一致，主要差别是部署目标不同：

- `Mini Portainer -arm`：面向 ARM Linux 设备，例如树莓派
- `Mini Portainer -×86`：面向 x86 Linux 主机，当前代码默认和服务端部署在同机

### 3. Web 控制台

目录：`mini-portainer-server/static`

前端为纯静态页面，使用：

- Vue 3 CDN
- Ionicons CDN

页面能力：

- 展示各节点在线状态
- 展示 CPU 温度、内存占用、容器列表
- 支持容器启动、停止、重启
- 支持查看容器日志快照
- 支持中英文切换

### 4. 工作流程

系统运行流程如下：

1. Agent 每 5 秒采集一次本机状态。
2. Agent 将状态 POST 到服务端 `/api/report`。
3. 服务端保存节点最新状态，并在响应中返回待执行命令。
4. Agent 如果收到 `start / stop / restart / logs` 命令，就立即在本机执行。
5. 前端页面通过 `/api/data` 轮询获取所有节点状态。
6. 当前端发起日志查看时，服务端先下发 `logs` 命令，Agent 再抓取最近日志并回传到服务端。

## 项目目录

```text
crossPlatformContainerMonitor/
├─ mini-portainer-server/      # 中心服务端和前端页面
│  ├─ main.go
│  ├─ go.mod
│  └─ static/
│     ├─ index.html
│     ├─ app.js
│     └─ style.css
├─ Mini Portainer -arm/        # ARM 设备 Agent
│  ├─ main.go
│  ├─ go.mod
│  ├─ go.sum
│  └─ task.md
└─ Mini Portainer -×86/        # x86 设备 Agent
   ├─ main.go
   ├─ go.mod
   ├─ go.sum
   └─ task.md
```

补充说明：

- `main.go`：各模块主入口
- `go.mod / go.sum`：Go 依赖定义
- `static/`：服务端托管的前端静态资源
- `task.md`：项目早期设计记录
- `agent_pi64`、`agent_aliyun`、`server_linux`：目录中已有的编译产物

<img width="1891" height="1021" alt="Mini Portainer Agent" src="https://github.com/user-attachments/assets/04196b7c-93a4-47f8-bc43-9c0a706c253f" />


## 使用说明

### 1. 环境要求

服务端：

- Go 1.25
- 可开放 `9000` 端口

Agent：

- Linux 环境
- 已安装并运行 Docker
- 当前用户可访问 Docker Socket
- Go 1.25

### 2. 构建项目

分别进入三个子目录独立构建。

构建服务端：

```powershell
cd mini-portainer-server
go build ./...
```

构建 ARM Agent：

```powershell
cd "Mini Portainer -arm"
go build ./...
```

构建 x86 Agent：

```powershell
cd "Mini Portainer -×86"
go build ./...
```

如果需要交叉编译，目录中的 `task.md` 已给出示例：

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o agent_aliyun main.go
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o agent_pi64 main.go
```

### 3. 配置 Agent 地址

启动前需要检查 Agent 中的服务端地址配置。

`Mini Portainer -arm/main.go` 中当前是占位符：

```go
const CloudServerURL = "http://你的公网IP:9000/api/report?node=RaspberryPi_3B"
```

部署前需要替换成实际的服务端地址。

`Mini Portainer -×86/main.go` 中当前默认配置为：

```go
const CloudServerURL = "http://127.0.0.1:9000/api/report?node=Aliyun_Server"
```

这表示它默认和服务端部署在同一台 Linux 主机。

### 4. 启动顺序

建议按下面顺序启动：

1. 启动服务端
2. 在目标设备上启动对应 Agent
3. 浏览器访问 `http://<服务端地址>:9000/`

### 5. 常用功能

页面打开后可以进行以下操作：

- 查看所有节点最近一次上报时间
- 查看节点 CPU 温度和内存占用
- 查看节点上的全部容器
- 对容器执行启动、停止、重启
- 查看容器最近日志快照

### 6. 主要接口

服务端当前提供这些接口：

- `POST /api/report?node=<nodeName>`：Agent 上报状态并领取命令
- `GET /api/data`：前端获取所有节点状态
- `POST /api/command?node=<node>&action=<action>&id=<containerId>`：前端下发控制命令
- `POST /api/logs/submit?node=<node>&id=<containerId>`：Agent 回传日志
- `GET /api/logs?node=<node>&id=<containerId>`：前端轮询读取日志

支持的控制命令：

- `start`
- `stop`
- `restart`
- `logs`

## 说明

这个项目当前实现已经能完成基础的跨节点容器监控和控制，适合家庭实验室、小型自用环境或作为 Go + Docker SDK 的练手项目。

当前实现偏轻量，服务端状态保存在内存中，日志读取为快照模式，且没有认证与权限控制。如果后续准备长期使用，建议优先补充认证、持久化和配置化能力。
