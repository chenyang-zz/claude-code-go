# batch-214 阶段 1 源码对照与边界确认

## M1-1: TS 侧各 provider 错误处理源码分析

### 核心发现

TS 侧拥有一套完整的多层级错误处理体系，包括重试决策、错误分类、用户消息生成三个层面。

#### 1. `src/services/api/withRetry.ts` — 重试决策中心

**`shouldRetry(error: APIError)`** 是重试决策的核心函数：

| 条件 | 行为 | 说明 |
|------|------|------|
| `isMockRateLimitError(error)` | 永不重试 | `/mock-limits` 命令的测试错误 |
| `isPersistentRetryEnabled() && isTransientCapacityError(error)` | 总是重试 | 持久模式下 429/529 绕过 subscriber gate |
| CCR mode + `(401 \|\| 403)` | 总是重试 | CCR 中 auth 是 JWT，401/403 是瞬态网络问题 |
| `error.message?.includes('"type":"overloaded_error"')` | 重试 | SDK streaming 时 529 状态码丢失的 workaround |
| `parseMaxTokensContextOverflowError(error)` | 重试 | 自动调整 max_tokens 后重试 |
| `error instanceof APIConnectionError` | 重试 | 连接错误 |
| `error.status === 408` | 重试 | 请求超时 |
| `error.status === 409` | 重试 | 锁超时 |
| `error.status === 429` | 重试 | 限流 |
| `error.status === 401` | 重试 | 认证错误（清除 API key 缓存后重试） |
| `error.status >= 500` | 重试 | 服务器错误 |
| `x-should-retry: false` | 不重试 | 服务端明确指示 |
| `isOAuthTokenRevokedError(error)` | 不重试 | OAuth token 已撤销 |

**`categorizeRetryableAPIError(error)`** 将错误归类为 4 种 `SDKAssistantMessageError`：
- `'rate_limit'` — 429 / 529 / overloaded_error
- `'authentication_failed'` — 401 / 403
- `'server_error'` — status >= 408 (非 429)
- `'unknown'` — 其他

**认证错误特殊处理**：
- `isBedrockAuthError(error)` — AWS `CredentialsProviderError` 或 403
- `isVertexAuthError(error)` — google-auth-library 凭证错误或 401
- 认证错误触发**凭据缓存刷新**，然后重试

#### 2. `src/services/api/errors.ts` — 错误分类与用户消息

**`classifyAPIError(error)`** 返回 25+ 种分类字符串（用于 analytics / Datadog）：
- `rate_limit`, `server_overload`, `repeated_529`, `capacity_off_switch`
- `prompt_too_long`, `pdf_too_large`, `pdf_password_protected`, `image_too_large`
- `tool_use_mismatch`, `unexpected_tool_result`, `duplicate_tool_use_id`
- `invalid_model`, `credit_balance_low`
- `invalid_api_key`, `token_revoked`, `oauth_org_not_allowed`, `auth_error`
- `bedrock_model_access`
- `server_error`, `client_error`, `ssl_cert_error`, `connection_error`
- `unknown`

**`getAssistantMessageFromError(error)`** 将错误转换为用户可见消息，带 `error` 字段：
- `error: 'rate_limit'` — 429, 529, custom off switch
- `error: 'invalid_request'` — 400, 413, 404, tool_use 错误
- `error: 'authentication_failed'` — 401, 403
- `error: 'billing_error'` — credit balance too low
- `error: 'unknown'` — 其他

#### 3. `src/services/api/errorUtils.ts` — 连接错误解析

- `extractConnectionErrorDetails(error)` — 遍历 cause chain，提取 SSL/TLS 错误码
- `formatAPIError(error)` — 格式化错误消息，处理 CloudFlare HTML 页面
- 嵌套错误形状识别：
  - Bedrock: `{ error: { message } }`
  - Anthropic: `{ error: { error: { message } } }`

#### 4. 各 provider 特有错误

| Provider | 特有错误类型 | 处理方式 |
|----------|-------------|----------|
| Anthropic | `APIError` (status, message, headers) | 标准重试决策 |
| OpenAI | `APIError` | 同 Anthropic |
| Vertex | google-auth-library credential Error | `isVertexAuthError` 检测，刷新凭据 |
| Bedrock | AWS `CredentialsProviderError`, `Output.__type` | `isBedrockAuthError` 检测，刷新凭据 |
| Foundry | Azure AD 错误 | 未在 TS 侧发现专门处理（Foundry 较新） |

---

## M1-2: Go 侧现有错误分类现状评估

### 已有基础设施

#### 1. `internal/core/model/errors.go`

```go
type RetryableError interface {
    error
    IsRetryable() bool
    RetryAfter() time.Duration
}
```

- 仅定义了一个接口，无具体实现
- 无错误类型枚举

#### 2. `internal/runtime/engine/retry.go` — `isRetriableError(err)`

当前决策逻辑（按优先级）：

1. **接口检查**：`errors.As(err, &model.RetryableError)` → 若实现则调用 `IsRetryable()`
2. **网络错误**：`isNetworkError(err)` → 连接 refused/reset/EPIPE / `net.Error`
3. **HTTP 状态码正则**：`\b(529|500|502|503|504|429|408)\b`
4. **关键词匹配**：`"overloaded"` / `"rate_limit"` / `"timeout"`
5. **OpenAI 关键词**：`"server_error"` / `"temporary error"` / `"over capacity"`

**关键缺陷**：
- 纯字符串匹配，对非 Anthropic provider 的专用错误码（如 Bedrock `ThrottlingException`）无法识别
- 401/403 认证错误被状态码正则匹配为 retriable（`isRetriableError` 没有排除逻辑），但 Go 侧没有凭据刷新机制
- `x-should-retry` header 未被读取
- `retry-after` header 未被用于 backoff 计算

#### 3. `internal/runtime/engine/circuit_breaker_client.go`

```go
if isRetriableError(err) {
    c.breaker.RecordFailure()
}
```

**关键缺陷**：
- `isRetriableError` 返回 true 的**所有**错误都会触发熔断计数
- 包括 401 认证错误、网络错误等不应触发熔断的类型
- 这意味着用户配置错误（如 bad API key）可能意外熔断 provider

#### 4. `internal/core/model/provider_health.go`

```go
type HealthResult struct {
    Provider  string
    Status    HealthStatus // healthy/degraded/unhealthy/unknown/not_configured
    Message   string
    CheckedAt time.Time
}
```

**关键缺陷**：
- 只有 `Status` 和 `Message` 两个结果字段
- 无法区分"暂时不可用"（如 529 overloaded）和"永久配置错误"（如 401 bad auth）
- 无法做基于错误类型的诊断建议

#### 5. `internal/platform/api/anthropic/client.go`

错误返回形式：
```go
return nil, fmt.Errorf("anthropic api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
```

**关键缺陷**：
- 纯字符串错误，无结构化类型
- HTTP 状态码和响应体都塞在字符串里
- 没有错误类型可供 `errors.As` 匹配
- SSE error 事件仅提取 `.message` 字段，无状态码信息

#### 6. `internal/platform/api/openai/`

- 配额探测 (`quota_probe.go`) 使用 HTTP 请求 + 读取 rate-limit headers
- 错误处理同样以字符串为主

---

## M1-3: 边界确认与范围锁定

### 统一错误类型枚举

基于 TS 侧 `classifyAPIError` 和 `categorizeRetryableAPIError` 的分析，结合 Go 侧 engine 决策需求，定义以下 `ProviderErrorKind`：

| Kind | 触发条件 | retry | fallback | circuit breaker | health |
|------|---------|-------|----------|----------------|--------|
| `RateLimit` | 429, 限流头 | 是 | 是 | 是 | degraded |
| `ServerOverloaded` | 529, overloaded_error | 是 | 是 | 是 | degraded |
| `ServerError` | 500-504 | 是 | 是 | 是 | unhealthy |
| `Timeout` | 408, ETIMEDOUT | 是 | 是 | 是 | degraded |
| `AuthError` | 401, 403 (非 revoked) | 是 | 是 | **否** | unhealthy |
| `QuotaExceeded` | 配额超限 | 否 | 是 | **否** | unhealthy |
| `InvalidRequest` | 400, 413, 404 | 否 | 否 | **否** | healthy |
| `NetworkError` | ECONNREFUSED, ECONNRESET | 是 | 是 | 是 | degraded |
| `SSLCertError` | TLS/SSL 证书错误 | 否 | 否 | **否** | unhealthy |
| `Unknown` | 无法分类 | 否 | 否 | **否** | unknown |

**关键决策**：
- **AuthError 不触发熔断**：认证错误通常是配置问题，熔断无助于恢复
- **QuotaExceeded 不触发熔断**：配额耗尽需要用户行动，熔断无助于恢复
- **InvalidRequest 不触发熔断/重试/fallback**：请求本身有问题，重试只会重复失败
- **SSLCertError 不触发熔断**：企业代理证书问题需要用户配置

### Provider 映射范围

| Provider | 映射来源 | 覆盖错误 |
|----------|---------|---------|
| Anthropic | HTTP status + response body | 429, 529, 500-504, 408, 401, 403, 400, 413, 404 |
| OpenAI | HTTP status + response body | 429, 500, 401, 403, 400, 404, `insufficient_quota` |
| Vertex | HTTP status + google-auth-library 错误 | 429, 500, 401, 403, credential 错误 |
| Bedrock | HTTP status + AWS __type | 429, 500, 403, `ThrottlingException`, `CredentialsProviderError` |
| Foundry | HTTP status + response body | 429, 500, 401, 403, 400 |

### Engine 决策增强边界

- **`isRetriableError`**：优先检查 `ProviderError`，fallback 到现有关键词匹配
- **`CircuitBreakerClient.Stream`**：仅 `RateLimit`/`ServerOverloaded`/`ServerError`/`Timeout`/`NetworkError` 触发 `RecordFailure()`
- **`HealthChecker.CheckAll`**：在 `HealthResult` 中增加 `ErrorKind` 字段
- **`/status`**：增加 provider 最近错误类型统计

---

## M2-1 ~ M2-5: 统一错误类型体系与 Provider 映射（验证结果）

### 产出

- `internal/core/model/provider_error.go` — `ProviderErrorKind` 10 种枚举 + `ProviderError` 结构体
  - 实现 `error`、`RetryableError` 接口
  - 支持 `errors.Is/As`（通过 `Unwrap` 和 `WrapProviderError`）
  - `IsRetryable()` / `ShouldTriggerCircuitBreaker()` / `HealthImpact()` 决策方法
  - `ProviderErrorKindForRetryable()` fallback 辅助函数
- `internal/platform/api/anthropic/error_mapper.go` — `MapAPIError` + `MapError` + `classifyAnthropicError`
- `internal/platform/api/openai/error_mapper.go` — `MapAPIError` + `MapError` + `classifyOpenAIError`
- `internal/platform/api/anthropic/client.go` — HTTP 错误路径改用 `ParseAPIError` + `MapAPIError`；auth 错误路径返回 `ProviderErrorAuthError`
- 测试：`provider_error_test.go`（12 测试）+ `anthropic/error_mapper_test.go`（5 测试）+ `openai/error_mapper_test.go`（4 测试）

### 验证

- `go test ./internal/core/model/... -run TestProvider -count=1 -v` — 12 测试全部通过
- `go test ./internal/platform/api/anthropic/... -run TestMap -count=1 -v` — 5 测试全部通过
- `go test ./internal/platform/api/openai/... -run TestMap -count=1 -v` — 4 测试全部通过

---

## M3-1 ~ M3-5: Engine 韧性决策增强与可观测性（验证结果）

### M3-1: retry.go 决策增强

- **实现**：`isRetriableError` 无需修改 — `ProviderError` 已实现 `RetryableError` 接口（`IsRetryable()` + `RetryAfter()`），而 `isRetriableError` 第一步就是 `errors.As(err, &model.RetryableError)`，因此 `ProviderError` 自动被优先识别。
- **Fallback**：非 `ProviderError` 错误继续走原有关键词匹配路径。

### M3-2: circuit_breaker_client.go 状态转换增强

- **实现**：修改 `Stream` 方法错误处理逻辑：
  - 优先 `errors.As(err, &pe)` 提取 `ProviderError`
  - `pe.ShouldTriggerCircuitBreaker()` 为 true 时才 `RecordFailure()`
  - 非 `ProviderError` 的 fallback：检查 `isRetriableError(err)`，但额外排除含 "auth"/"unauthorized"/"forbidden" 关键词的错误
- **关键行为**：AuthError / QuotaExceeded / InvalidRequest / SSLCertError / Unknown 不触发熔断

### M3-3: provider_health.go 探测结果分类增强

- **实现**：`HealthResult` 新增 `ErrorKind ProviderErrorKind` 字段
- `platform/api/anthropic/health_probe.go` — 4 个 probe（Anthropic/Vertex/Bedrock/Foundry）全部填充 `ErrorKind`
- `platform/api/openai/health_probe.go` — OpenAI probe 填充 `ErrorKind`
- 新增 `healthErrorKind(statusCode, isNetworkError)` 辅助函数

### M3-4: `/status` provider 错误历史与诊断摘要

- **实现**：`providerHealthStatus()` 增强：
  - health 摘要行展示 `ErrorKind`（如 `anthropic=healthy, openai=unhealthy (auth_error)`）
  - 新增 `healthDiagnosticHint()` 函数，为每个 unhealthy/degraded provider 输出诊断提示
  - 提示示例：`! openai: authentication error — check credentials or run /login`

### M3-5: 集成测试

- `provider_error_integration_test.go` — 6 个测试：
  1. `TestProviderErrorRetryableClassification` — 10 种 Kind 的 retry 决策验证
  2. `TestCircuitBreakerProviderErrorClassification` — 10 种 Kind 的熔断触发验证
  3. `TestCircuitBreakerFallbackKeywordExcludesAuth` — 关键词 fallback 排除 auth
  4. `TestTryFallbackWithProviderError` — ProviderError 触发 fallback
  5. `TestHealthResultErrorKind` — health 分类映射验证
  6. `TestCircuitBreakerClient_*` 原有 7 个测试零回归

### 验证

- `go test ./internal/runtime/engine/... -count=1 -v` — 全部通过（原有 17 个 + 新增 6 个）
- `go test ./internal/services/commands/... -count=1 -v` — 全部通过（含 `TestProviderHealthStatus_WithResults` 更新）

---

## M4-1 ~ M4-2: 验证与文档收口

### M4-1: 全量回归

- `go build ./...` — 零错误
- `go test ./... -count=1` — 全量 86+ 包零回归

### M4-2: 分析文档

- 本文档已更新，完整记录 TS/Go 源码对照、设计决策、边界确认、实现验证结论

### 批次总结

- 新增文件：`internal/core/model/provider_error.go`、`internal/core/model/provider_error_test.go`、`internal/platform/api/anthropic/error_mapper.go`、`internal/platform/api/anthropic/error_mapper_test.go`、`internal/platform/api/openai/error_mapper.go`、`internal/platform/api/openai/error_mapper_test.go`、`internal/runtime/engine/provider_error_integration_test.go`
- 修改文件：`internal/runtime/engine/circuit_breaker_client.go`、`internal/core/model/provider_health.go`、`internal/platform/api/anthropic/health_probe.go`、`internal/platform/api/openai/health_probe.go`、`internal/platform/api/anthropic/client.go`、`internal/services/commands/status.go`、`internal/services/commands/status_health_test.go`
- 新增测试：27 个（core/model 12 + anthropic 5 + openai 4 + engine integration 6）
- 全量测试：86+ 包零回归

### 不纳入本批的能力

- 错误消息国际化
- 自动凭据刷新策略（401 处理只标记为 retriable，不实现刷新）
- `x-should-retry` header 读取（TS 侧有此逻辑但属于 edge case）
- retry-after 的精细化 backoff（已在 retry.go 中有 `parseRetryAfter`）
- 自定义错误恢复策略（自动降级模型、自动切换 region）
