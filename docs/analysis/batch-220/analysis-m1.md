# batch-220 阶段 1 源码对照与边界确认

## M1-1: TS 侧 OpenAI-Compatible Provider 主路径源码分析

### 关键发现：TS 侧不存在 OpenAI-Compatible Provider 实现

- `src/utils/model/providers.ts` 中的 `APIProvider` 类型仅包含：`'firstParty' | 'bedrock' | 'vertex' | 'foundry'`
- 不存在 `openai`、`openai-compatible` 或 `glm` 选项
- TS 侧所有 API 调用均通过 `@anthropic-ai/sdk` 完成（`src/services/api/client.ts`）
- `getAPIProvider()` 函数仅根据环境变量选择 Bedrock/Vertex/Foundry/FirstParty

### 结论

Go 侧的 OpenAI-Compatible Provider（含 GLM）是迁移项目的**新增能力**，并非从 TS 侧迁移而来。因此本批次的"源码对照"实际上是：
1. 确认 TS 侧无对应参考实现
2. 基于 OpenAI API 官方规范和通用兼容层惯例，验证 Go 侧实现是否正确
3. 确保 Go 侧实现与 engine 运行时抽象（`core/model` 接口）正确集成

## M2-2: Go 侧 `internal/platform/api/openai/` 现状梳理

### 文件清单与功能

| 文件 | 行数 | 功能 | 测试数 |
|------|------|------|--------|
| `client.go` | 529 | Chat Completions API 客户端（SSE 流、请求构建、响应解析） | 7 |
| `responses_client.go` | 273 | Responses API 客户端（OpenAI o1/o3 系列） | 0（通过 responses_mapper_test 间接覆盖） |
| `responses_mapper.go` | 188 | Responses API 请求/响应映射 | 7 |
| `responses_types.go` | 148 | Responses API 类型定义 | 3 |
| `responses_router.go` | 27 | Chat Completions vs Responses API 路由选择 | 1 |
| `mapper.go` | 3 | 占位（暂无独立逻辑） | 0 |
| `error_mapper.go` | 123 | 错误映射到统一 ProviderError | 4 |
| `errors.go` | 229 | APIError 定义、解析、分类 | 13 |
| `health_probe.go` | 118 | 健康探测（HTTP GET 探测） | 5 |
| `quota_probe.go` | 150 | 配额探测（usage/limit 头解析） | 8 |
| `status_probe.go` | 81 | 状态探测 | 2 |

**总计：50 个单元测试**

### Bootstrap 集成现状

`DefaultEngineFactory` 中 `ProviderOpenAICompatible` / `ProviderGLM` 分支（app.go:1012-1070）：
- 正确创建 `openai.Client` 或 `openai.ResponsesClient`
- 正确装配 `engine.Runtime`（toolExecutor、toolCatalog、approvalService、hookRunner 等）
- 正确注册 Agent tool 和 WebSearch tool
- 调用 `applyOpenAIAdvancedDefaults` 读取环境变量
- 已有 bootstrap 测试覆盖：`TestDefaultEngineFactoryBuildsOpenAICompatibleRuntime`、`TestDefaultEngineFactoryBuildsGLMRuntime`

### 已验证通过的能力

1. **请求构建**：messages 映射、tool 定义映射、stream_options、max_tokens 映射
2. **SSE 流解析**：text delta、tool_calls 增量解析、finish_reason 映射
3. **错误处理**：HTTP 错误解析、错误类型分类、retry-after 解析
4. **Bootstrap 装配**：完整的 engine runtime 构建

## M1-3: 边界确认与最小范围锁定

### 已确认缺口清单

| 优先级 | 缺口 | 位置 | 影响 |
|--------|------|------|------|
| P1 | `max_completion_tokens` 与 `max_tokens` 同时发送 | `client.go:184-191` | 某些兼容 provider 可能拒绝同时包含两个字段的请求 |
| P1 | `tool_use` 后无 `tool_result` 映射到 assistant message | `client.go:mapMessages` | tool loop 第二轮请求格式不正确 |
| P2 | `[DONE]` 事件与 `finish_reason=="tool_calls"` 双重触发 `emitToolUses` | `client.go:handleChunk` | 可能导致重复 tool use 事件 |
| P2 | `streamUsageOption` 对非官方 OpenAI URL 返回 nil | `client.go:510-515` | 兼容 provider 无法获取 usage 信息 |
| P3 | 缺少完整对话循环集成测试 | - | 无法验证端到端主路径 |
| P3 | Responses API tool result 回写格式为自定义 XML | `responses_mapper.go:97-101` | 可能与 Responses API 期望格式不一致 |

### 阶段 2 修复范围锁定

**纳入修复：**
1. P1: 修复 `max_completion_tokens`/`max_tokens` 发送逻辑（优先 `max_completion_tokens`，兼容层 fallback）
2. P1: 修复 tool loop 中第二轮请求的 message 映射（assistant 需包含 `tool_calls`，user 需包含 `tool` role 结果）
3. P2: 修复 `[DONE]` 与 `tool_calls` 双重触发问题

**延后处理：**
- Responses API tool result 格式（需要更深入研究 Responses API 规范）
- 高级参数（temperature、top_p、reasoning 等）

### 阶段 3 验证范围

- 最小对话循环集成测试（HTTP mock，无工具调用）
- Tool use 集成测试（单工具调用、结果回写）
- 错误恢复集成测试（rate limit、invalid model）
