# Architecture

## Repository Map

下面这份目录图以当前仓库实际结构为准，用来回答“代码现在放在哪里”。它和后面的“分层职责”是配套看的：

```text
claude-code-go/
├── cmd/
│   └── cc/                       # 进程入口
├── configs/
│   ├── default.yaml
│   └── examples/local.yaml
├── docs/
│   ├── architecture.md
│   ├── engine.md
│   └── tools.md
├── internal/
│   ├── app/
│   │   ├── bootstrap/            # 启动装配:NewAppFromArgs / EngineAssembly
│   │   └── wiring/               # 模块依赖注入
│   ├── core/                     # 语义内核(稳定抽象)
│   │   ├── agent/                # agent / team / agent 间消息领域
│   │   ├── command/              # 命令抽象与注册表接口
│   │   ├── compact/              # 上下文压缩(boundary、estimate、prompt)
│   │   ├── config/               # 配置对象、加载与合并规则
│   │   ├── conversation/         # 会话请求/响应/历史抽象
│   │   ├── event/                # 流式输出事件模型
│   │   ├── featureflag/          # 特性开关(env: CLAUDE_FEATURE_*)
│   │   ├── hook/                 # 钩子配置、matcher、事件类型
│   │   ├── message/              # 消息、内容块、角色
│   │   ├── model/                # 模型客户端接口与流抽象
│   │   ├── permission/           # 权限请求、规则、决策
│   │   ├── session/              # session 聚合、快照、仓储接口
│   │   ├── task/                 # TodoV2 任务列表数据模型
│   │   ├── tool/                 # 工具接口、调用结构、注册表接口
│   │   └── transcript/           # 会话录像 JSONL 条目模型
│   ├── platform/                 # 外部系统接入(server/client/adapter)
│   │   ├── api/
│   │   │   ├── anthropic/        # Anthropic API 客户端
│   │   │   │   └── betas/        # Beta 能力组合(capabilities/composer)
│   │   │   └── openai/           # OpenAI 兼容客户端
│   │   ├── config/
│   │   ├── fs/                   # 文件系统适配
│   │   ├── git/                  # git CLI 包装
│   │   ├── lsp/                  # Language Server 进程管理与 JSON-RPC
│   │   ├── mailbox/              # 团队邮箱(文件 + flock)
│   │   ├── mcp/
│   │   │   ├── bridge/           # MCP 协议→内部对象桥接(含 elicitation)
│   │   │   ├── client/           # MCP 客户端与传输层
│   │   │   └── registry/         # MCP server 注册表
│   │   ├── oauth/                # Claude.ai OAuth 流程(token/refresh/listener)
│   │   ├── plugin/               # plugin 发现、加载、注册、监听
│   │   ├── remote/               # 远端会话(SSE/WS)与订阅管理
│   │   ├── shell/                # shell 执行抽象
│   │   ├── store/
│   │   │   └── sqlite/           # 会话 / 任务持久化
│   │   ├── team/                 # 团队配置与成员状态
│   │   └── telemetry/            # 日志与指标
│   ├── runtime/                  # 执行编排
│   │   ├── approval/             # 工具调用审批
│   │   ├── coordinator/          # 多代理协调器与调度
│   │   ├── cron/                 # 定时任务调度器(jitter/lock)
│   │   ├── engine/               # 对话主循环、工具循环、post-turn hook
│   │   ├── executor/             # 命令/工具执行器
│   │   ├── hooks/                # hook matcher 与命令型 hook 执行
│   │   ├── repl/                 # CLI 解析、唤醒、远端桥接
│   │   └── session/              # 会话生命周期、自动保存、后台任务
│   ├── services/                 # 产品能力层(用户可见的命令/工具/后台服务)
│   │   ├── autodream/            # 后台记忆整合(去重、剪枝)
│   │   ├── claudeailimits/       # Claude.ai 速率限制观测
│   │   ├── commands/             # 内置 slash 命令(/login、/mcp、/model 等)
│   │   ├── extractmemories/      # 后台记忆抽取(从对话历史)
│   │   ├── magicdocs/            # 自动文档更新(# MAGIC DOC: 检测)
│   │   ├── policylimits/         # 组织级策略限制
│   │   ├── prompts/              # 系统提示词与工具说明文档拼装
│   │   ├── promptsuggestion/     # 提示词建议生成
│   │   ├── settingssync/         # 用户设置远端同步
│   │   ├── tips/                 # 启动期 tips 推送
│   │   └── tools/                # 内置 Tool 实现集合(详见下文)
│   └── ui/
│       ├── console/              # 终端渲染、approval、stream renderer
│       ├── jsonout/              # JSON 输出
│       └── tui/                  # 交互式 TUI(Bubble Tea 风格)
├── pkg/
│   ├── logger/                   # 基于 zerolog 的日志包
│   └── sdk/                      # 对外暴露的 SDK 边界
├── migrations/                   # SQLite 迁移脚本
├── scripts/
└── testdata/
```

可以先按这个粗粒度理解：

- `cmd` 是启动入口。
- `internal/app` 是装配根。
- `internal/core` 是语义内核。
- `internal/platform` 是外部系统接入层，这里也包含各类 server/client/adapter。
- `internal/runtime` 是执行编排层。
- `internal/services` 是产品能力层，含一组**长期驻留的后台服务**(extractmemories、autodream、tips 等)和**用户可见的命令/工具**。
- `internal/ui` 是展示层。
- `pkg/logger` 是跨层共用的日志组件,基于 zerolog。
- `pkg/sdk` 是对外暴露的 SDK 边界，不属于内部运行时主干。
- `migrations` 存放 SQLite schema 迁移,由 `internal/platform/store/sqlite` 在启动期消费。

## Layers

- `cmd`
  - 进程入口，只负责启动应用和处理最外层参数。
- `internal/app`
  - 组装根。把 `core` 定义的接口、`platform` 提供的适配、`services` 提供的能力装配成一个可运行的应用。
  - `bootstrap.NewAppFromArgs` 是真正的入口,负责解析 CLI、加载配置、初始化模型/store/registry/MCP/plugin/oauth/cron 等并组装出 `EngineAssembly`。
- `internal/core`
  - 最稳定的一层，放领域模型、抽象接口和跨实现共享的协议。
  - 这里描述"系统是什么"，不描述"系统跑在哪"。
- `internal/runtime`
  - 执行编排层。负责把一次 CLI 交互、一次模型请求、一次工具调用组织成完整流程。
  - 这里处理顺序、重试、事件流、审批、调度，但不直接绑定某个外部厂商。
  - `runtime/cron` 提供独立于会话的定时调度器,通过 `platform/store` 读取持久化任务,与 `services/tools/cron` 工具配对使用。
  - `runtime/coordinator` 实现多代理协调,与 `services/tools/agent` 一起为 sub-agent / team 模式提供执行支持。
- `internal/platform`
  - 外部系统适配层。负责 Anthropic/OpenAI API、文件系统、shell、git、sqlite、MCP、plugin、telemetry、LSP、OAuth、邮箱、远端会话等宿主能力接入。
  - 其中 `internal/platform/mcp` 负责把外部 MCP server 的协议消息转成内部可消费的 client、bridge 与 registry 能力，包括 request/notification 分发和协议桥接。
  - `internal/platform/oauth` 把 Claude.ai 登录的 PKCE 流程、token 刷新、监听端口等收敛在一处,对外只暴露 `service` 接口。
  - `internal/platform/lsp` 负责 LSP 进程生命周期、JSON-RPC 收发、诊断结果聚合,被 `services/tools/lsp` 包装成产品工具。
  - `internal/platform/team` + `internal/platform/mailbox` 共同支撑多代理团队的元数据与消息存储。
  - 这里解决"怎么跟外界说话"。
- `internal/services`
  - 内建产品能力层。把 `core` 抽象和 `platform` 能力组合成用户可见的命令、工具、提示词,以及一组**后台服务**。
  - 后台服务通常有自己的 `Init`/`InitOptions`,在 `app/bootstrap` 中按 feature flag 或登录态决定是否启用,典型成员见下文 [Built-in Services](#built-in-services)。
  - 这里解决"产品要提供什么能力"。
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
- `runtime/hooks` 可以在不引入 UI 的前提下承接命令型 hook 执行与 matcher 过滤；当前已支持按 MCP server name 过滤 elicitation 相关 hooks。

这样做的原因很直接：

- `core` 稳定，方便先把概念和边界立住，再逐步替换实现。
- `platform` 易变，外部 API、协议、认证、存储都会变化，必须隔离。
- `runtime` 专门承接流程复杂度，避免把调用顺序、重试、审批散落到各个工具和 UI。
- `services` 专门承接产品语义，避免把“这是一个 bash tool”这类产品概念塞回 `platform`。
- `app` 统一装配，可以避免全局单例和隐式依赖蔓延。

## What Is Core

`core` 不是"公共工具箱"，而是系统最小、最稳定的语义内核。当前目录基本已经体现出这一点：

- `internal/core/message`
  - 消息、内容块、角色等基础对话结构。
- `internal/core/conversation`
  - 一轮或多轮对话请求/响应/历史的抽象。
- `internal/core/event`
  - 流式输出事件模型，供 engine、UI、日志统一消费。
- `internal/core/model`
  - 模型客户端接口与流抽象,例如 `Client`、`Stream`、`Usage`。
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
- `internal/core/hook`
  - hook 配置、matcher、事件类型(PreToolUse/PostToolUse/UserPromptSubmit/SubagentStart/StatusChange/PermissionDenied 等)。
- `internal/core/task`
  - TodoV2 任务列表的数据模型(`Task`、`Status`)与 `Store` 接口,支撑 task_create / task_update / task_list 工具。
- `internal/core/transcript`
  - 会话录像 JSONL 条目模型(`UserEntry`、`AssistantEntry`、`ToolUseEntry` 等)与读写器、恢复逻辑。
- `internal/core/compact`
  - 上下文压缩抽象:压缩边界、压缩前后 token 估算、压缩提示词组装。
- `internal/core/featureflag`
  - 特性开关常量与读取约定。所有开关通过 `CLAUDE_FEATURE_<NAME>` 环境变量驱动,例如 `TODO_V2`、`TOKEN_BUDGET`、`EXTRACT_MEMORIES`、`AUTO_DREAM`、`PROMPT_SUGGESTION`、`COORDINATOR_MODE` 等。

判断一个东西应不应该放进 `core`，可以看三点：

- 它是不是一个稳定概念，而不是某个供应商或协议的细节。
- 它能不能脱离 Anthropic、OpenAI、MCP、Shell、SQLite 这些具体实现独立存在。
- 它是否值得被多个实现复用。

例如：

- `tool.Tool` 在 `core` 是对的，因为"不管工具来自本地实现还是 MCP 代理，它最终都得像一个工具"。
- `model.Client` 在 `core` 是对的，因为"不管底层接 Anthropic 还是 OpenAI，运行时看到的都是一个可流式返回的模型客户端"。
- `transcript.Entry` 在 `core` 是对的,因为录像格式必须能跨 UI、跨存储、跨恢复路径复用。
- `featureflag.Flag*` 常量在 `core` 是对的,因为它只是开关名字的字面值,而读取/绑定逻辑由各使用方自己决定。
- `ServerRegistry` 不应放进 `core`，因为"server"已经带有具体接入协议和宿主环境语义。

## What Is Server

这里的 `server` 不是一个单独架构层，而是一类外部能力提供者。当前代码里，"server"主要有这几种含义：

- MCP server
  - 典型位置：`internal/platform/mcp/registry`、`internal/platform/mcp/client`、`internal/platform/mcp/bridge`
  - 它们代表外部工具/资源/提示词提供方，通常通过 MCP 协议暴露能力。
  - 这里也包括 MCP 控制面消息的处理，例如 `elicitation/create` 请求和 `notifications/elicitation/complete` 通知。
- API server / model provider
  - 典型位置：`internal/platform/api/anthropic`(含 `betas/` 子包,负责 Beta header 组合)、`internal/platform/api/openai`
  - 它们是远程模型服务端，提供补全、流式响应等能力。
- LSP server
  - 典型位置:`internal/platform/lsp`
  - 它们是按需启动的语言服务进程,通过 JSON-RPC 提供诊断、补全等能力,被 `services/tools/lsp` 包装成产品工具。
- OAuth server / Claude.ai 远端
  - 典型位置:`internal/platform/oauth`、`internal/platform/remote`
  - `oauth` 负责把用户带向 Claude.ai 完成授权,本地起一个 listener 接 callback;`remote` 负责后续 SSE/WS 长连接、订阅与远端 subagent 事件。
- plugin 带入的 server
  - 典型位置：`internal/platform/plugin`
  - plugin 可能声明或装配新的 MCP server、agent、命令、hook、LSP server,本质仍是外部能力来源。

所以"哪些是 servers"更准确地说，是：

- 任何需要网络、进程、协议、鉴权、连接管理才能访问的能力提供方，都更像 `platform` 里的 server/client/adapter。
- 它们可以被注册、发现、代理、桥接，但它们本身不属于 `core`。

如果从目录上看，你会发现 `platform` 和 `core` 是平铺在 `internal/` 下的，这没有问题。这里表达的是：

- `server` 不应再单独拉出一个和 `core`、`runtime` 并列的新架构层。
- `server` 相关实现应该收敛在 `platform` 这类外部接入层里。
- 它与 `core` "同级目录"，但不与 `core` 共享同一种职责。

## Core 和 Server 的关系

两者关系应该是：

- `core` 定义系统需要什么抽象。
- `server` 在 `platform` 中实现或提供这些抽象背后的真实能力。
- `runtime` 负责编排调用时机。
- `services` 把这些能力包装成真正暴露给用户的命令和工具。

以 MCP 为例：

1. `core/tool` 定义“工具”应该长什么样。
2. `platform/mcp/client` 负责连接外部 MCP server。
3. `platform/mcp/bridge` 把 MCP 协议对象转换成系统内部可调用对象，并把 elicitation 这类控制面请求接到 hook 生命周期。
4. `services/tools/mcp` 把代理后的能力作为产品级 tool 暴露出去。
5. `runtime/hooks` 负责执行命令型 hooks，`runtime/engine` 在对话过程中决定何时调用它。
6. `app/bootstrap` 在客户端连接建立后注册 MCP bridge，使 request、notification 和 hooks 能形成闭环。

这样分层的好处是：

- 替换 server 接入方式时，不必重写对话核心。
- 本地工具和远程 MCP 工具可以共享一套 runtime 和权限模型。
- UI 不需要知道工具来自本地实现还是远端 server。

## Request Flow

一次典型执行路径可以概括为：

1. `cmd` 接收 CLI 输入并启动应用。
2. `internal/app` 完成配置加载、依赖装配、注册表初始化、后台服务 `Init`、定时器启动。
3. `internal/runtime/repl` 解析输入并驱动 `engine`。
4. `internal/runtime/engine` 基于 `core` 的请求、事件、工具抽象执行主循环;在主循环边界派发 `UserPromptSubmit`、`PreToolUse`、`PostToolUse`、`PostTurn` 等 hook 事件。
5. `internal/services` 提供具体命令、工具、提示词,并通过 hook 注入后台服务(extractmemories、autodream、magicdocs、promptsuggestion 等)。
6. 如需访问外部资源，则通过 `internal/platform` 的 adapter/client/server 完成。
7. 若外部 MCP server 触发 `elicitation/create`，则 `platform/mcp/client` 分发 request，`platform/mcp/bridge` 先执行 hooks 再默认回退，`runtime/hooks` 根据 server name 做匹配，最后将结果写回协议。
8. 模型/工具调用结果通过 `core/transcript` 写入会话录像;UI 通过 `core/event` 流消费实时事件。
9. 当上下文逼近窗口上限,`runtime/engine` 借助 `core/compact` 触发上下文压缩,生成 boundary + summary,然后继续主循环。

## Built-in Services

`internal/services` 下除了 `commands`、`prompts`、`tools` 这三个常驻包,还有一组**后台服务**。它们大多在 `app/bootstrap` 启动期通过 `Init(opts InitOptions)` 模式接入,按 feature flag 或登录态决定是否运行:

| 包 | 作用 | 触发方式 |
| --- | --- | --- |
| `services/extractmemories` | 每个 turn 结束后,fork 一个 sub-agent 把对话中的稳定事实写入 `~/.claude/.../memory/` | `CLAUDE_FEATURE_EXTRACT_MEMORIES` + post-turn hook |
| `services/autodream` | 周期性扫描 memory 目录,合并、去重、剪枝,保持索引精简 | `CLAUDE_FEATURE_AUTO_DREAM` + 时间/会话双闸门 |
| `services/magicdocs` | 文件读取时检测 `# MAGIC DOC:` 头部,turn 结束后让 sub-agent 更新对应文档 | `FileReadListener` + post-turn hook |
| `services/promptsuggestion` | 在交互空闲期生成下一步 prompt 建议,用于 UI 展示 | feature flag + 推断模型 |
| `services/settingssync` | 增量上传/下载用户级 `~/.claude/settings.json` 等 | 登录后台 goroutine,差异同步 |
| `services/claudeailimits` | 解析 Claude.ai 订阅响应头,记录速率/额度并预警 | 模型客户端中间件 + 持久化 |
| `services/policylimits` | 拉取组织策略文件(模型黑白名单、能力上限),按周期刷新 | 后台 poller + 缓存 |
| `services/tips` | 启动期根据使用情况推送 tips,带 cooldown 与 history | `services/commands/init` 调用 |

设计要点:

- 每个后台服务都有自己的 `featureflag.go` 或 `eligibility.go`,显式回答"在什么条件下我才应该跑"。
- 它们只读 `core` 抽象、写入 `platform` 设施(store/oauth/fs),不直接改写 UI。
- 想新增一个类似的服务,先想清楚:它是被 hook 触发,还是周期性轮询,还是请求中间件;然后参考现有命名(`init.go` + `Init(opts)` + 私有状态)。

## Built-in Tools Catalog

`internal/services/tools` 是产品工具的实际仓库,工具数量较多,按类别组织如下(每个工具都实现 `core/tool.Tool` 接口,通过 `runtime/executor` 调度):

- 文件与代码
  - `file_read`、`file_write`、`file_edit`、`notebook_edit`、`glob`、`grep`、`lsp`
- 进程与系统
  - `bash`、`sleep`、`worktree/{enter,exit}`
- 任务与协作
  - `todo_write`、`task_{create,get,list,update,output,stop}`、`team_{create,delete}`、`send_message`
  - 内部基于 `core/task` 与 `platform/mailbox` / `platform/team`
- 调度与远程
  - `cron/{create,delete,list}`(配合 `runtime/cron`)、`schedule_wakeup`、`remote_trigger`
- 用户交互
  - `ask_user_question`、`enter_plan_mode`、`exit_plan_mode`、`brief`
- 元能力
  - `tool_search`、`config_tool`、`skill`(+ `bundled/` 内的 `claude_api`、`debug`、`loop`、`remember`、`simplify`、`update_config` 等)
  - `agent`(子 agent 调用) + `agent/builtin`(`general_purpose`、`explore`、`plan`、`claude_code_guide`、`statusline_setup`、`verification`)+ `agent/loader`(用户/插件/管理员 agent 加载)+ `agent/memory`(agent 记忆快照)
- 网络
  - `web_fetch`、`web_search`
- MCP 代理
  - `mcp/`、`mcp/list_resources`、`mcp/read_resource`(把 `platform/mcp/bridge` 暴露的能力包成 Tool)
- 输出辅助
  - `synthetic_output`、`shared/`(图片处理、JSON schema 工具等)

新增工具的最小三件套:

1. 在 `services/tools/<name>/` 实现 `Tool` 接口和参数 schema。
2. 在 `services/prompts/<name>_prompt.go` 编写工具说明文档(被注入系统提示)。
3. 在 `app/bootstrap/registry.go` 或 `wiring/modules.go` 完成注册;若工具依赖外部能力,先在 `platform/` 准备好 adapter,再在工具实现里引用。

## Practical Placement Guide

新代码落点可以按这个标准判断：

- 如果你在定义稳定接口、消息结构、权限决策、事件模型，放 `core`。
- 如果你在写执行顺序、重试、调度、审批流程，放 `runtime`。
- 如果你在接外部 API、MCP、shell、git、sqlite、plugin、LSP、OAuth、远端会话,放 `platform`。
- 如果你在做 MCP request / notification 分发、elicitation bridge 或连接级协议适配，放 `platform/mcp`。
- 如果你在做 hook matcher、hook 输入输出模型，放 `core/hook` 与 `runtime/hooks`。
- 如果你在实现 `/login`、`/mcp`、`BashTool`、`FileReadTool` 这类产品能力，放 `services`。
- 如果你在做后台周期任务、post-turn hook、sub-agent 触发的能力(memory、tips、magic docs),放 `services/<name>` 并提供 `Init(opts)` 入口。
- 如果你要加一个 feature flag,只往 `core/featureflag` 加常量,不要在 core 里读 env;读取由消费方负责。
- 如果你在引入新的录像条目类型,放 `core/transcript` 并扩展 reader/writer。
- 如果你在做上下文压缩相关的边界、估算、提示词策略,放 `core/compact`,具体的触发时机由 `runtime/engine` 决定。
- 如果你在做启动装配和依赖注入，放 `app`。
- 如果你在做 console/TUI/json 输出，放 `ui`。
- 如果你在写一个跨层共享的纯工具(目前只有日志),放 `pkg/`,并保持零依赖于 `internal/`。

一句话总结：

- `core` 是"系统的语义中心"。
- `server` 是"系统接入的外部能力提供方"，通常属于 `platform`，不是与 `core` 并列的层。
- `services` 既包含用户可见的命令和工具,也包含按 feature flag 启停的后台服务。
- `platform` 和 `core` 可以是同级目录，但不应被理解为同一种抽象层。
