# Migration Plan

1. 建立最小闭环
2. 接入真实 model client
3. 增加 session store
4. 增加 read-only tools
5. 增加 approval 与 file/bash tools
6. 接入 MCP 与 plugin

## 第一批工具迁移承接目录

第一批迁移沿用当前骨架，不再新建额外顶层目录。承接关系固定如下：

- `internal/core/tool`
  - 最小工具接口、调用结构、结果结构、注册表接口
- `internal/core/permission`
  - 工具读写权限请求、规则、决策模型
- `internal/platform/fs`
  - 本地文件系统访问、路径归一化、沙箱边界、errno 适配
- `internal/services/tools/glob`
  - `GlobTool` 实现
- `internal/services/tools/grep`
  - `GrepTool` 实现
- `internal/services/tools/file_read`
  - `FileReadTool` 实现
- `internal/services/tools/file_write`
  - `FileWriteTool` 实现
- `internal/services/tools/file_edit`
  - `FileEditTool` 实现
- `internal/services/tools/shared`
  - 工具共用辅助逻辑，例如输入解析、输出格式化、diff 文本生成
- `internal/app/wiring`
  - 汇总注册表、依赖装配、工具实例注册
- `internal/runtime/executor`
  - 统一执行入口、输出截断和执行时约束

约束：

- `core` 保持纯接口和领域对象，不下沉宿主实现。
- `platform` 只处理底层适配，不感知具体工具语义。
- `services/tools/*` 负责每个工具的业务逻辑和结果拼装。
- `app/wiring` 是注册入口，不承担工具实现。
- 后续新增工具继续放在 `internal/services/tools/*`，不再扩散新命名空间。

## 工具注册与调用入口

与恢复版 TS 宿主的映射关系固定如下：

- TS `src/tools.ts`
  - Go 对应 `internal/app/wiring` + `internal/core/tool.Registry`
  - 含义：统一创建工具实例并注册到注册表，不把注册逻辑散落到具体工具包。
- TS `src/query.ts`
  - Go 对应 `internal/runtime/engine/tool_cycle.go`
  - 含义：接住模型返回的 tool call，并驱动一轮工具执行循环。
- TS `src/services/tools/toolExecution.ts`
  - Go 对应 `internal/runtime/executor/tool_executor.go`
  - 含义：按名称取工具、执行输入解码/校验、做权限与限制检查，然后调用工具。
- TS `src/services/tools/toolOrchestration.ts` 与 `src/services/tools/StreamingToolExecutor.ts`
  - 第一批先不单独建等价层。
  - 含义：先完成单批次统一执行；并发调度、流式执行、进度消息后补。

当前迁移要求：

- 所有本地工具和后续 MCP 代理工具，都必须最终走同一套 `Registry -> ToolCycle -> ToolExecutor -> tool.Tool.Invoke(...)` 链路。
- `internal/services/tools/*` 只写单工具逻辑，不负责注册表写入，也不直接驱动执行循环。
