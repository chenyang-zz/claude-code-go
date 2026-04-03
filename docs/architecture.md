# Architecture

## Repository Map

下面这份目录图以当前仓库实际结构为准，用来回答“代码现在放在哪里”。它和后面的“分层职责”是配套看的：

```text
claude-code-go/
├── cmd/
│   └── cc/
│       └── main.go
├── configs/
│   ├── default.yaml
│   └── examples/
│       └── local.yaml
├── docs/
│   ├── architecture.md
│   ├── engine.md
│   ├── migration-plan.md
│   └── tools.md
├── internal/
│   ├── app/
│   │   ├── bootstrap/
│   │   └── wiring/
│   ├── core/
│   │   ├── agent/
│   │   ├── command/
│   │   ├── config/
│   │   ├── conversation/
│   │   ├── event/
│   │   ├── message/
│   │   ├── model/
│   │   ├── permission/
│   │   ├── session/
│   │   └── tool/
│   ├── platform/
│   │   ├── api/
│   │   │   ├── anthropic/
│   │   │   └── openai/
│   │   ├── config/
│   │   ├── fs/
│   │   ├── git/
│   │   ├── mcp/
│   │   │   ├── bridge/
│   │   │   ├── client/
│   │   │   └── registry/
│   │   ├── plugin/
│   │   ├── remote/
│   │   ├── shell/
│   │   ├── store/
│   │   │   └── sqlite/
│   │   └── telemetry/
│   ├── runtime/
│   │   ├── approval/
│   │   ├── coordinator/
│   │   ├── engine/
│   │   ├── executor/
│   │   ├── repl/
│   │   └── session/
│   ├── services/
│   │   ├── commands/
│   │   ├── prompts/
│   │   └── tools/
│   │       ├── agent/
│   │       ├── bash/
│   │       ├── file_edit/
│   │       ├── file_read/
│   │       ├── glob/
│   │       ├── grep/
│   │       └── mcp/
│   └── ui/
│       ├── console/
│       ├── jsonout/
│       └── tui/
├── migrations/
├── pkg/
│   └── sdk/
├── scripts/
└── testdata/
```

可以先按这个粗粒度理解：

- `cmd` 是启动入口。
- `internal/app` 是装配根。
- `internal/core` 是语义内核。
- `internal/platform` 是外部系统接入层，这里也包含各类 server/client/adapter。
- `internal/runtime` 是执行编排层。
- `internal/services` 是产品能力层。
- `internal/ui` 是展示层。
- `pkg/sdk` 是对外暴露的 SDK 边界，不属于内部运行时主干。

## Layers

- `cmd`
  - 进程入口，只负责启动应用和处理最外层参数。
- `internal/app`
  - 组装根。把 `core` 定义的接口、`platform` 提供的适配、`services` 提供的能力装配成一个可运行的应用。
- `internal/core`
  - 最稳定的一层，放领域模型、抽象接口和跨实现共享的协议。
  - 这里描述“系统是什么”，不描述“系统跑在哪”。
- `internal/runtime`
  - 执行编排层。负责把一次 CLI 交互、一次模型请求、一次工具调用组织成完整流程。
  - 这里处理顺序、重试、事件流、审批、调度，但不直接绑定某个外部厂商。
- `internal/platform`
  - 外部系统适配层。负责 Anthropic/OpenAI API、文件系统、shell、git、sqlite、MCP、plugin、telemetry 等宿主能力接入。
  - 这里解决“怎么跟外界说话”。
- `internal/services`
  - 内建产品能力层。把 `core` 抽象和 `platform` 能力组合成用户可见的命令、工具、提示词等功能。
  - 这里解决“产品要提供什么能力”。
- `internal/ui`
  - 展示与交互层。负责 console、json 输出、TUI 等不同表现形式。

## Dependency Rule

依赖方向应当尽量保持为：

`cmd -> app -> runtime/services -> core`

这里要特别区分两件事：

- 目录结构上的“同级”
- 架构职责上的“分层”

在当前仓库里，`internal/core`、`internal/platform`、`internal/runtime`、`internal/services` 确实都是 `internal` 下的同级目录；这只是为了代码组织清晰，不表示它们在抽象上处于同一层级，也不表示它们可以随意双向依赖。

这里说的“层”，更准确是依赖规则和职责边界：

- `core` 更靠内，负责稳定语义。
- `platform` 更靠外，负责外部系统接入。
- 两者虽然目录同级，但在架构上不是对等可互相渗透的关系。

同时：

- `runtime` 和 `services` 可以依赖 `core`。
- `app` 可以依赖所有层，因为它就是装配根。
- `platform` 可以实现 `core` 中定义的接口，但不应该反过来把自己的类型泄漏进 `core`。
- `ui` 可以依赖 `runtime`、`services`、`core`，但不应成为业务决策中心。
- `core` 不依赖 `platform` 或 `ui`，也尽量不依赖具体第三方 SDK。

这样做的原因很直接：

- `core` 稳定，方便先把概念和边界立住，再逐步替换实现。
- `platform` 易变，外部 API、协议、认证、存储都会变化，必须隔离。
- `runtime` 专门承接流程复杂度，避免把调用顺序、重试、审批散落到各个工具和 UI。
- `services` 专门承接产品语义，避免把“这是一个 bash tool”这类产品概念塞回 `platform`。
- `app` 统一装配，可以避免全局单例和隐式依赖蔓延。

## What Is Core

`core` 不是“公共工具箱”，而是系统最小、最稳定的语义内核。当前目录基本已经体现出这一点：

- `internal/core/message`
  - 消息、内容块、角色等基础对话结构。
- `internal/core/conversation`
  - 一轮或多轮对话请求/响应/历史的抽象。
- `internal/core/event`
  - 流式输出事件模型，供 engine、UI、日志统一消费。
- `internal/core/model`
  - 模型客户端接口与流抽象，例如 `Client`、`Stream`。
- `internal/core/tool`
  - 工具接口、调用结构、结果结构、注册表接口。
- `internal/core/command`
  - 命令抽象与注册表接口。
- `internal/core/permission`
  - 权限请求、规则、决策模型。
- `internal/core/session`
  - session 聚合、快照、仓储接口。
- `internal/core/agent`
  - agent、team、agent 间消息等多代理领域对象。
- `internal/core/config`
  - 配置对象、加载接口、合并规则。

判断一个东西应不应该放进 `core`，可以看三点：

- 它是不是一个稳定概念，而不是某个供应商或协议的细节。
- 它能不能脱离 Anthropic、OpenAI、MCP、Shell、SQLite 这些具体实现独立存在。
- 它是否值得被多个实现复用。

例如：

- `tool.Tool` 在 `core` 是对的，因为“不管工具来自本地实现还是 MCP 代理，它最终都得像一个工具”。
- `model.Client` 在 `core` 是对的，因为“不管底层接 Anthropic 还是 OpenAI，运行时看到的都是一个可流式返回的模型客户端”。
- `ServerRegistry` 不应放进 `core`，因为“server”已经带有具体接入协议和宿主环境语义。

## What Is Server

这里的 `server` 不是一个单独架构层，而是一类外部能力提供者。当前代码里，“server”主要有这几种含义：

- MCP server
  - 典型位置：`internal/platform/mcp/registry`、`internal/platform/mcp/client`
  - 它们代表外部工具/资源提供方，通常通过 MCP 协议暴露能力。
- API server / model provider
  - 典型位置：`internal/platform/api/anthropic`、`internal/platform/api/openai`
  - 它们是远程模型服务端，提供补全、流式响应等能力。
- plugin 带入的 server
  - 典型位置：`internal/platform/plugin`
  - plugin 可能声明或装配新的 MCP server，本质仍是外部能力来源。

所以“哪些是 servers”更准确地说，是：

- 任何需要网络、进程、协议、鉴权、连接管理才能访问的能力提供方，都更像 `platform` 里的 server/client/adapter。
- 它们可以被注册、发现、代理、桥接，但它们本身不属于 `core`。

如果从目录上看，你会发现 `platform` 和 `core` 是平铺在 `internal/` 下的，这没有问题。这里表达的是：

- `server` 不应再单独拉出一个和 `core`、`runtime` 并列的新架构层。
- `server` 相关实现应该收敛在 `platform` 这类外部接入层里。
- 它与 `core` “同级目录”，但不与 `core` 共享同一种职责。

## Core 和 Server 的关系

两者关系应该是：

- `core` 定义系统需要什么抽象。
- `server` 在 `platform` 中实现或提供这些抽象背后的真实能力。
- `runtime` 负责编排调用时机。
- `services` 把这些能力包装成真正暴露给用户的命令和工具。

以 MCP 为例：

1. `core/tool` 定义“工具”应该长什么样。
2. `platform/mcp/client` 负责连接外部 MCP server。
3. `platform/mcp/bridge` 把 MCP 协议对象转换成系统内部可调用对象。
4. `services/tools/mcp` 把代理后的能力作为产品级 tool 暴露出去。
5. `runtime/engine` 在对话过程中决定何时调用它。

这样分层的好处是：

- 替换 server 接入方式时，不必重写对话核心。
- 本地工具和远程 MCP 工具可以共享一套 runtime 和权限模型。
- UI 不需要知道工具来自本地实现还是远端 server。

## Request Flow

一次典型执行路径可以概括为：

1. `cmd` 接收 CLI 输入并启动应用。
2. `internal/app` 完成配置加载、依赖装配、注册表初始化。
3. `internal/runtime/repl` 解析输入并驱动 `engine`。
4. `internal/runtime/engine` 基于 `core` 的请求、事件、工具抽象执行主循环。
5. `internal/services` 提供具体命令、工具、提示词。
6. 如需访问外部资源，则通过 `internal/platform` 的 adapter/client/server 完成。
7. 结果以 `core/event` 流的形式交给 `internal/ui` 渲染。

## Practical Placement Guide

新代码落点可以按这个标准判断：

- 如果你在定义稳定接口、消息结构、权限决策、事件模型，放 `core`。
- 如果你在写执行顺序、重试、调度、审批流程，放 `runtime`。
- 如果你在接外部 API、MCP、shell、git、sqlite、plugin，放 `platform`。
- 如果你在实现 `/login`、`/mcp`、`BashTool`、`FileReadTool` 这类产品能力，放 `services`。
- 如果你在做启动装配和依赖注入，放 `app`。
- 如果你在做 console/TUI/json 输出，放 `ui`。

一句话总结：

- `core` 是“系统的语义中心”。
- `server` 是“系统接入的外部能力提供方”，通常属于 `platform`，不是与 `core` 并列的层。
- `platform` 和 `core` 可以是同级目录，但不应被理解为同一种抽象层。
