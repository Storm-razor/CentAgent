# CentAgent 项目目录结构说明

本项目遵循 Go 语言标准项目布局 (Standard Go Project Layout)，并结合 CLI 工具与 Agent 开发的特性进行组织。

## 核心目录

### `/cmd`
项目的入口文件存放地。
- `/cmd/centagent`: 主程序入口，包含 `main.go`。负责初始化 CLI 框架 (Cobra) 和加载配置。

### `/internal`
项目的私有应用代码库，此目录下的代码无法被外部项目导入，确保封装性。

- `/internal/agent`
  - 核心智能体逻辑。
  - 包含 Eino 的 Graph/Chain 定义、Prompt 模板管理、以及 Agent 的编排逻辑。
  
- `/internal/cli`
  - 命令行接口定义层。
  - 基于 Cobra 定义各种命令 (如 `chat`, `start`, `ps`) 及其参数解析。
  - 负责将用户输入的 CLI 指令路由到对应的 Handler。

- `/internal/tui`
  - 终端用户界面 (Terminal User Interface) 层。
  - 基于 Bubbletea/Huh 构建。
  - 包含交互式组件（加载动画、确认框、表格展示）的实现。

- `/internal/docker`
  - Docker 引擎交互层。
  - 封装 Docker SDK，提供操作容器、镜像、网络的原子能力 (Tools)。
  - 这里的函数将被 Agent 层作为 Tool 调用。

- `/internal/monitor`
  - 监控与后台任务层。
  - 负责定时采集容器状态 (CPU/Mem)，处理异常告警。
  - 包含定时任务调度器 (Ticker/Cron)。

- `/internal/storage`
  - 数据持久化层。
  - 负责 SQLite 数据库的初始化、Migration 以及 GORM Model 定义。
  - 提供日志和监控数据的 CRUD 接口。

- `/internal/config`
  - 配置管理。
  - 定义配置结构体，处理配置文件读取 (Viper) 和默认值设置。

- `/internal/utils`
  - 通用工具函数库。
  - 如日志格式化、时间处理、公共常量等。

## 其他目录

### `/configs`
- 存放默认配置文件模板 (如 `config.yaml`)。

### `/scripts`
- 构建、安装、测试脚本 (如 `Makefile`, `build.sh`)。

### `/docs`
- 项目文档，包括设计文档 (`Design.md`)、需求文档 (`WiN.md`) 等。

## Docker 部署与使用教程

本项目支持以容器方式运行，并通过挂载宿主机 Docker Socket 来监视整台机器上的 Docker 容器系统。

### 1) 前置条件

- 已安装 Docker（Docker Desktop 或 Linux Docker Engine）
- Docker 运行在 Linux 容器模式（Docker Desktop 默认通常是 Linux engine）
- 在项目根目录执行命令（包含 `docker-compose.yml` 的目录）

### 2) 准备配置与密钥

1. 复制环境变量模板并填入必填项（不要把密钥写入镜像或提交到仓库）：

```powershell
copy .env.example .env
notepad .env
```

`.env` 至少需要配置：

- `ARK_API_KEY=...`
- `ARK_MODEL_ID=...`

可选（加速构建）：

- `REGISTRY_MIRROR=docker.m.daocloud.io`（基础镜像 registry 加速）
- `ALPINE_MIRROR=https://mirrors.aliyun.com/alpine`（apk 包仓库加速）
- `GOPROXY=https://goproxy.cn,direct`（go mod 下载加速）

2. （可选）按需修改本地配置文件：

- 本地文件：`.\configs\config.yaml`
- 容器内挂载位置：`/etc/centagent/config.yaml`（只读）

### 3) 启动（后台常驻运行监控）

在项目根目录执行：

```powershell
docker compose up -d
```

该命令会：

- 启动容器后默认执行 `centagent start --config /etc/centagent/config.yaml`
- 通过挂载 `/var/run/docker.sock` 连接宿主机 Docker
- 将数据库持久化到命名数据卷 `centagent-data`（容器内路径 `/var/lib/centagent`）

### 3.1) （可选）手动构建镜像（包含完整参数示例）

如果你希望先单独构建镜像再启动容器，可以使用 `docker build` 或 `docker compose build`。

1. 使用 `docker build`（PowerShell，推荐把换行写成反引号）：

```powershell
docker build `
  --build-arg REGISTRY=docker.m.daocloud.io `
  --build-arg ALPINE_MIRROR=https://mirrors.aliyun.com/alpine `
  --build-arg GOPROXY=https://goproxy.cn,direct `
  -t centagent:local .
```

参数说明：

- `REGISTRY`：基础镜像来源（解决 `FROM alpine/golang` 拉取慢/失败）
- `ALPINE_MIRROR`：apk 包仓库镜像（解决 `apk add` 慢）
- `GOPROXY`：Go 模块代理（解决 `go mod download` 慢，并避免依赖系统 git）

2. 使用 `docker compose build`（读取 `.env` 中的 `REGISTRY_MIRROR/ALPINE_MIRROR/GOPROXY`）：

```powershell
docker compose build
docker compose up -d
```

如果你已经构建好镜像，不想每次 `up` 触发构建，可以：

```powershell
docker compose up -d --no-build
```

### 4) 验证是否运行正常

```powershell
docker ps
docker logs -f centagent
```

如果需要确认容器内读取到的配置文件内容：

```powershell
docker exec -it centagent cat /etc/centagent/config.yaml
```

确认数据库文件是否已生成/增长：

```powershell
docker exec -it centagent ls -lh /var/lib/centagent
```

### 5) 部署后如何使用（进入容器执行其他命令）

容器会常驻跑监控；当你需要交互使用其它命令时，通过 `docker exec` 进入：

- 运行交互式聊天（TUI）：

```powershell
docker exec -it centagent centagent chat --ui=tui
```

- 或进入 shell 后自行执行命令：

```powershell
docker exec -it centagent sh
```

### 5.1) centagent 子命令使用教程（在运行中的容器内）

CentAgent 的命令结构是：

- 根命令：`centagent`
- 全局参数：`--config <path>`（建议在容器内始终使用 `/etc/centagent/config.yaml`）
- 子命令：`start`、`chat`、`storage ...`

下面所有示例都假设你的容器名为 `centagent`，并显式指定配置文件：

#### 5.1.1 查看帮助与版本信息

```powershell
docker exec -it centagent centagent --help
docker exec -it centagent centagent start --help
docker exec -it centagent centagent chat --help
docker exec -it centagent centagent storage --help
```

#### 5.1.2 start：启动监控服务（常驻）

通常不需要你手动运行（容器启动时默认会执行）。如果你想在容器内手动启动（例如调试），可以：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml start
```

注意：该命令会阻塞前台运行，按 Ctrl+C 停止；作为容器主进程运行时由 Docker 管理生命周期。

#### 5.1.3 chat：交互式对话（console / tui）

默认是 console UI：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml chat
```

使用 TUI（推荐）：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml chat --ui=tui
```

关闭工具调用前确认（不建议长期关闭，调试时可用）：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml chat --confirm-tools=false
```

#### 5.1.4 storage：数据库概况与数据维护

查看数据库统计概况（stats/logs/audit 计数、DB 文件大小等）：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml storage info
```

立即执行一次“监控数据精简”（忽略定时任务间隔，按配置里的 retention 策略清理）：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml storage prune-monitor
```

清理审计记录（二选一：按条数或按天数）：

```powershell
docker exec -it centagent centagent --config /etc/centagent/config.yaml storage prune-audit --keep 1000
docker exec -it centagent centagent --config /etc/centagent/config.yaml storage prune-audit --days 7
```

### 6) 修改配置后的生效方式

由于配置文件是挂载到容器内并在启动时读取，修改 `.\configs\config.yaml` 后，建议重启容器让 `start` 重新加载：

```powershell
docker compose restart centagent
```

## Windows 本地构建与运行（Makefile）

如果你希望不使用 Docker、直接在 Windows 上构建并运行，可以使用项目内置的 `Makefile`。

### 1) 前置条件

- 已安装 Go
- 已安装 `make`（例如使用 Git Bash / MSYS2 / Chocolatey 提供的 make）

### 2) 设置必需环境变量（当前 PowerShell 会话）

```powershell
$env:ARK_API_KEY="replace_me"
$env:ARK_MODEL_ID="replace_me"
```

### 3) 构建与运行

构建二进制到 `.\bin\centagent.exe`：

```powershell
make build
```

以前台方式启动监控（使用默认配置 `CONFIG=./configs/config.yaml`）：

```powershell
make start
```

启动聊天（TUI）：

```powershell
make chat-tui
```

查看数据库概况：

```powershell
make storage-info
```

如果你有自定义配置文件路径，可以覆盖 `CONFIG`：

```powershell
make start CONFIG=./configs/config.yaml
make chat-tui CONFIG=./configs/config.yaml
```
