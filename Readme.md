# CentAgent 项目目录结构说明

本项目遵循 Go 语言标准项目布局 (Standard Go Project Layout)，并结合 CLI 工具与 Agent 开发的特性进行组织。
可部署运行的程序请见次级分支,主分支仍在开发中

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
