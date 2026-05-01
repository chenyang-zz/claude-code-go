# batch-212 分析文档：Provider 运行时韧性 — 跨 Provider Fallback 与健康探测

## §1 TS 侧源码分析

### 1.1 Provider 选择与模型切换

TS 侧的 provider 选择是**启动时静态决定**的，没有运行时切换能力。

**`src/utils/model/providers.ts`**（41 行）：
- `getAPIProvider()` 根据环境变量返回 `'firstParty' | 'bedrock' | 'vertex' | 'foundry'`
- 选择逻辑：`CLAUDE_CODE_USE_BEDROCK` → `CLAUDE_CODE_USE_VERTEX` → `CLAUDE_CODE_USE_FOUNDRY` → `'firstParty'`
- 一旦选定，整个会话期间不会变更

**`src/services/api/client.ts`**（299+ 行）：
- `getAnthropicClient()` 是工厂函数，根据 `getAPIProvider()` 的结果创建对应 SDK client
- Bedrock → `@anthropic-ai/bedrock-sdk` 的 `AnthropicBedrock`
- Foundry → `@anthropic-ai/foundry-sdk` 的 `AnthropicFoundry`
- Vertex → `@anthropic-ai/vertex-sdk` 的 `AnthropicVertex`
- FirstParty → `@anthropic-ai/sdk` 的 `Anthropic`
- 所有返回类型都通过 `as unknown as Anthropic` 伪装成统一类型
- **没有运行时 provider 切换或 fallback 机制**

### 1.2 Retry 与 Fallback 逻辑

**`src/services/api/withRetry.ts`**（823 行）：
- `withRetry<T>()` 是核心 retry 循环，封装所有 API 调用
- 支持 `fallbackModel` 参数，但 fallback 是**同 provider 内切换模型**（如 Opus → Sonnet），不是跨 provider
- `FallbackTriggeredError` 在连续 529 错误达到 `MAX_529_RETRIES`（3 次）时抛出
- 有复杂的 fast mode fallback、persistent retry、auth 错误处理等逻辑
- **没有跨 provider 的健康探测或切换逻辑**

**`src/services/api/errors.ts`**（500+ 行）：
- 包含错误分类（`is529Error`、`shouldRetry` 等）
- 处理各种 provider 特定的错误（AWS credentials、GCP credentials、OAuth）
- **没有跨 provider 错误分类**

### 1.3 关键结论

- TS 侧**不存在跨 provider fallback 机制**，所有 fallback 都是同 provider 内的模型切换
- TS 侧**不存在 provider 健康探测机制**
- Provider 选择是启动时静态决定，运行时不可变更
- batch-212 的"跨 Provider Fallback"是 Go 侧的**增量能力**，不是从 TS 侧直接迁移的能力

---

## §2 Go 侧现状评估

### 2.1 现有 Fallback 基础设施（batch-67）

**`runtime/engine/retry.go`**：
- `isRetriableError(err)` — 基于 `model.RetryableError` 接口、HTTP 状态码（529/500/502/503/504/429/408）、关键词匹配判断错误是否可重试
- `fallbackResult` — 包含 `model` 和 `stream`
- `tryFallback(ctx, req, primaryErr)` — 在**同一个 `Client` 上切换 `Model` 字段**，调用 `e.Client.Stream(ctx, fbReq)`
- `shouldFallbackAfterAttempts(attempt)` — 基于 `FallbackAfterAttempts` 控制 fallback 时机

**`runtime/engine/engine.go`** `streamAndConsume()`：
- 连接错误和流错误均会触发 retry + fallback 路径
- 每次重试前检查 `shouldFallbackAfterAttempts()`
- 所有重试耗尽后调用 `tryFallback()`
- 成功 fallback 后发射 `event.TypeModelFallback` 事件

### 2.2 现有健康探测

**`platform/api/anthropic/status_probe.go`**：
- `StatusProbe` 结构体，HTTP GET `/v1/messages` 端点
- 3 秒超时，返回 `APIConnectivityProbeResult{Summary}`
- 使用配置的 `APIBaseURL` 和 `APIKey`

**`platform/api/openai/status_probe.go`**：
- `StatusProbe` 结构体，HTTP GET 聊天补全端点
- 3 秒超时，返回 `APIConnectivityProbeResult{Summary}`
- 使用配置的 `APIBaseURL` 和 `APIKey`

**关键观察**：
- 现有 status probe 是**独立的**、**非接口化的**，每个 provider 有自己的实现
- probe 结果只有文本摘要，没有结构化健康状态
- probe 在 `/status` 命令中调用，不在 Engine 运行时中调用

### 2.3 现有 Provider Client 结构

**`core/model/client.go`**：
```go
type Client interface {
    Stream(ctx context.Context, req Request) (Stream, error)
}
```
- 极简接口，只有 `Stream` 方法
- 所有 provider（Anthropic / OpenAI / Vertex / Bedrock / Foundry）都实现了此接口

**现有 provider client 目录**：
- `platform/api/anthropic/client.go` — Anthropic 原生 + Vertex + Bedrock + Foundry 扩展
- `platform/api/openai/client.go` — OpenAI + Responses API

### 2.4 复用策略

- **复用 `model.RetryableError` 接口** — 已有 `IsRetryable()` + `RetryAfter()`
- **复用 `RetryPolicy` 结构** — 已有指数退避 + jitter
- **复用 `fallbackResult` 模式** — 已有 `model` + `stream` 的封装
- **复用 `streamAndConsume()` 的 retry 框架** — 在 Engine 层注入跨 provider fallback
- **扩展现有 status probe** — 为 Vertex / Bedrock / Foundry 增加 probe 实现

---

## §3 边界确认与范围锁定

### 3.1 纳入范围

| 能力 | 理由 |
|------|------|
| 统一 ProviderHealth 接口 | 将现有零散的 status probe 抽象为统一接口，支持运行时健康检查 |
| 5 个 provider 健康探测实现 | Anthropic / OpenAI 已有，需补 Vertex / Bedrock / Foundry |
| Fallback 决策引擎 | 基于 `model.RetryableError` 和 provider 优先级列表的切换逻辑 |
| Engine Stream 路径 fallback 注入 | 复用现有 `streamAndConsume()` 框架，扩展为跨 provider |
| `/status` provider 健康摘要 | 在现有 `/status` 输出中增加结构化健康状态 |

### 3.2 排除范围

| 能力 | 排除理由 |
|------|----------|
| Circuit breaker | 需要失败计数器、半开状态、时间窗口等复杂状态机，超出"最小闭环" |
| 复杂负载均衡 | 需要延迟/成本/配额多维度评分、权重路由，需要额外基础设施 |
| Provider 动态热切换 | 需要运行时重新创建 client 和配置，涉及 session 状态一致性 |
| 跨 provider 会话状态同步 | 不同 provider 的消息格式差异使会话共享成本过高 |
| 基于成本/延迟的智能路由 | 需要成本模型与延迟观测基础设施 |

### 3.3 设计决策

1. **ProviderHealth 接口放在 `core/model`** — 健康探测是稳定的运行时概念，不依赖具体 provider
2. **Fallback 决策放在 `runtime/engine`** — fallback 是运行时编排行为，不是 provider 内部逻辑
3. **健康探测复用现有 HTTP client** — 各 provider 已有 `HTTPClient` 配置，probe 直接使用
4. **Provider 优先级由 bootstrap 配置决定** — 默认只有一个 provider 激活，fallback 列表由配置扩展
5. **不修改 `model.Client` 接口** — 保持接口稳定，跨 provider 切换在 Engine 层通过多 client 管理实现
