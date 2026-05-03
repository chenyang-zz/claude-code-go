# claude-code-go

`claude-code-go` 是 [Claude Code](https://claude.ai/code)（原 `@anthropic-ai/claude-code` CLI）的 Go 语言重新实现。目标是提供一个高性能、可移植、架构清晰的 AI 编程助手运行时，同时保持与原 TypeScript/Ink 版本一致的产品体验。

## 特性

- **对话引擎** — 模型流式补全、工具调用循环、多轮对话管理
- **工具系统** — 统一的 Tool 接口，支持 Bash、文件读写、Grep、Glob、MCP 代理等
- **权限与安全** — 可配置的审批策略、权限规则与决策模型
- **Hook 系统** — UserPromptSubmit、SubagentStart、WorktreeCreate/Remove 等生命周期钩子
- **Agent 与 Team** — 多 Agent 协作、团队管理、记忆注册表
- **MCP 协议** — MCP server 注册、发现、桥接与客户端接入
- **Plugin 系统** — 可扩展的插件架构
- **多 UI 输出** — 支持 Console、TUI（终端 UI）、JSON 流式输出
- **多模型后端** — 支持 Anthropic API、OpenAI 兼容 API
- **后台服务** — OAuth 自动刷新、记忆提取、MagicDocs 检测、Policy Limits、Rate Limit 观测等

## 快速开始

### 前置条件

- Go 1.26+

### 安装与运行

```bash
# 克隆仓库
git clone https://github.com/sheepzhao/claude-code-go.git
cd claude-code-go

# 编译并运行
go run ./cmd/cc
```

### 配置

通过 `configs/default.yaml` 进行运行时配置：

```yaml
app:
  name: claude-code-go
runtime:
  provider: anthropic      # 模型提供商：anthropic / openai
  model: claude-sonnet      # 默认模型
  approval_mode: ask        # 审批模式：ask / auto / yolo
```

更多配置示例见 [configs/examples/](configs/examples/)。

## 架构

项目采用分层架构，依赖方向为 `cmd → app → runtime/services → core`：

```text
cmd/cc/                          # 进程入口
internal/
├── app/                         # 装配根（bootstrap + wiring）
│   ├── bootstrap/               # 启动引导、配置加载、依赖装配
│   └── wiring/                  # 模块注册与依赖注入
├── core/                        # 语义内核（稳定抽象层）
│   ├── agent/                   # Agent/Team 领域模型、记忆注册表
│   ├── command/                 # 命令抽象与注册表
│   ├── compact/                 # 上下文压缩
│   ├── config/                  # 配置对象与加载接口
│   ├── conversation/            # 多轮对话抽象
│   ├── event/                   # 流式事件模型
│   ├── featureflag/             # 特性开关
│   ├── hook/                    # Hook 事件定义
│   ├── message/                 # 消息结构（content block / role）
│   ├── model/                   # 模型客户端接口
│   ├── permission/              # 权限请求与规则
│   ├── session/                 # Session 聚合与仓储
│   ├── task/                    # 任务定义
│   ├── tool/                    # 工具接口与注册表
│   └── transcript/              # 对话转录
├── runtime/                     # 执行编排层
│   ├── approval/                # 审批流程
│   ├── coordinator/             # 协作协调
│   ├── cron/                    # 定时任务
│   ├── engine/                  # 对话执行引擎（主循环）
│   ├── executor/                # 工具执行调度
│   ├── hooks/                   # Hook 执行与匹配
│   ├── repl/                    # REPL 输入驱动
│   └── session/                 # 运行时 Session
├── platform/                    # 外部系统适配层
│   ├── api/                     # 模型 API 接入（Anthropic / OpenAI）
│   ├── config/                  # 文件系统配置
│   ├── fs/                      # 文件系统操作
│   ├── git/                     # Git 集成
│   ├── lsp/                     # LSP 集成
│   ├── mailbox/                 # 消息信箱
│   ├── mcp/                     # MCP 协议（client / bridge / registry）
│   ├── oauth/                   # OAuth 认证
│   ├── plugin/                  # 插件系统
│   ├── remote/                  # 远程会话
│   ├── shell/                   # Shell 执行
│   ├── store/                   # 持久化存储（SQLite）
│   ├── team/                    # 团队管理
│   └── telemetry/               # 遥测
├── services/                    # 产品能力层
│   ├── autodream/               # 后台记忆整合
│   ├── claudeailimits/          # Rate Limit 观测
│   ├── commands/                # 内建命令
│   ├── extractmemories/         # 记忆提取
│   ├── magicdocs/               # Magic Docs 检测
│   ├── policylimits/            # 组织策略限制
│   ├── prompts/                 # 提示词系统
│   ├── promptsuggestion/        # 提示建议
│   ├── settingssync/            # 设置同步
│   ├── tips/                    # 操作提示
│   └── tools/                   # 内建工具实现
└── ui/                          # 展示层
    ├── console/                 # 控制台输出
    ├── jsonout/                 # JSON 流式输出
    └── tui/                     # 终端 UI（TUI）
pkg/
├── logger/                      # 结构化日志（zerolog）
└── sdk/                         # 对外 SDK 边界
```

各层职责详见 [docs/architecture.md](docs/architecture.md)。

### 一次典型执行流

1. `cmd/cc` 接收 CLI 输入并启动应用
2. `internal/app` 完成配置加载、依赖装配、注册表初始化
3. `internal/runtime/repl` 解析输入并驱动 `engine`
4. `internal/runtime/engine` 基于 core 的抽象执行主循环（模型调用 → 工具调用 → 结果流式输出）
5. `internal/services` 提供具体命令、工具、提示词实现
6. 外部资源访问通过 `internal/platform` 的 adapter/client 完成
7. 结果以 `core/event` 流形式交给 `internal/ui` 渲染

## 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/rs/zerolog` | 结构化日志 |
| `github.com/gorilla/websocket` | WebSocket 通信 |
| `modernc.org/sqlite` | 纯 Go SQLite 存储 |
| `github.com/google/uuid` | UUID 生成 |
| `github.com/fsnotify/fsnotify` | 文件系统监听 |
| `github.com/stretchr/testify` | 测试断言 |
| `gopkg.in/yaml.v3` | YAML 配置解析 |

## 开发

```bash
# 运行所有测试
go test ./...

# 编译
go build ./cmd/cc

# 代码检查
go vet ./...
```

更多开发指南见 [docs/](docs/) 目录与 [AGENTS.md](AGENTS.md)。
