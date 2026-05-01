# batch-211 分析文档：Foundry Provider 最小骨架

## §1 TS 侧 Foundry Provider 源码分析

### 1.1 Provider 检测与模型选择

**文件**：`src/utils/model/providers.ts`

```typescript
export type APIProvider = 'firstParty' | 'bedrock' | 'vertex' | 'foundry'

export function getAPIProvider(): APIProvider {
  return isEnvTruthy(process.env.CLAUDE_CODE_USE_BEDROCK)
    ? 'bedrock'
    : isEnvTruthy(process.env.CLAUDE_CODE_USE_VERTEX)
      ? 'vertex'
      : isEnvTruthy(process.env.CLAUDE_CODE_USE_FOUNDRY)
        ? 'foundry'
        : 'firstParty'
}
```

Foundry 通过 `CLAUDE_CODE_USE_FOUNDRY` 环境变量启用，优先级次于 Bedrock 和 Vertex。

**文件**：`src/utils/model/configs.ts`

Foundry 的模型 ID 直接使用模型名本身，无特殊格式转换：

```typescript
export const CLAUDE_3_7_SONNET_CONFIG = {
  firstParty: 'claude-3-7-sonnet-20250219',
  bedrock: 'us.anthropic.claude-3-7-sonnet-20250219-v1:0',
  vertex: 'claude-3-7-sonnet@20250219',
  foundry: 'claude-3-7-sonnet',
} as const satisfies ModelConfig
```

所有 Foundry 模型 ID 均为简单模型名（如 `claude-opus-4-6`、`claude-sonnet-4-5`），**无需映射表**，直接透传即可。

### 1.2 客户端创建

**文件**：`src/services/api/client.ts` (lines 191-220)

```typescript
if (isEnvTruthy(process.env.CLAUDE_CODE_USE_FOUNDRY)) {
  const { AnthropicFoundry } = await import('@anthropic-ai/foundry-sdk')
  // Determine Azure AD token provider based on configuration
  // SDK reads ANTHROPIC_FOUNDRY_API_KEY by default
  let azureADTokenProvider: (() => Promise<string>) | undefined
  if (!process.env.ANTHROPIC_FOUNDRY_API_KEY) {
    if (isEnvTruthy(process.env.CLAUDE_CODE_SKIP_FOUNDRY_AUTH)) {
      // Mock token provider for testing/proxy scenarios
      azureADTokenProvider = () => Promise.resolve('')
    } else {
      // Use real Azure AD authentication with DefaultAzureCredential
      const { DefaultAzureCredential: AzureCredential, getBearerTokenProvider } =
        await import('@azure/identity')
      azureADTokenProvider = getBearerTokenProvider(
        new AzureCredential(),
        'https://cognitiveservices.azure.com/.default',
      )
    }
  }

  const foundryArgs: ConstructorParameters<typeof AnthropicFoundry>[0] = {
    ...ARGS,
    ...(azureADTokenProvider && { azureADTokenProvider }),
    ...(isDebugToStdErr() && { logger: createStderrLogger() }),
  }
  return new AnthropicFoundry(foundryArgs) as unknown as Anthropic
}
```

**关键发现**：

1. **SDK 封装**：`AnthropicFoundry` 是 `@anthropic-ai/foundry-sdk` 提供的类，底层仍使用 Anthropic Messages API 协议。
2. **端点构建**：由 SDK 内部处理，基于 `ANTHROPIC_FOUNDRY_RESOURCE` 或 `ANTHROPIC_FOUNDRY_BASE_URL`：
   - `https://{resource}.services.ai.azure.com/anthropic/v1/messages`
3. **认证模式（三种）**：
   - **API Key**：`ANTHROPIC_FOUNDRY_API_KEY` → SDK 默认读取
   - **Azure AD**：无 API Key 且未 Skip 时，使用 `DefaultAzureCredential` + `getBearerTokenProvider`
   - **Skip Auth**：`CLAUDE_CODE_SKIP_FOUNDRY_AUTH` → mock token provider
4. **消息格式**：与 Anthropic API 完全相同（SDK 继承 Anthropic 接口）。
5. **Beta header 差异**：1P/Foundry 使用 `advanced-tool-use`，Vertex/Bedrock 使用 `tool-search-tool`（line 1175）。当前 Go 侧 engine 已统一处理 beta header，本批不纳入此差异。

### 1.3 端点与环境变量

**文件**：`src/services/api/client.ts` (lines 43-53)

```typescript
/**
 * Foundry (Azure):
 * - ANTHROPIC_FOUNDRY_RESOURCE: Your Azure resource name (e.g., 'my-resource')
 *   For the full endpoint: https://{resource}.services.ai.azure.com/anthropic/v1/messages
 * - ANTHROPIC_FOUNDRY_BASE_URL: Optional. Alternative to resource - provide full base URL directly
 *   (e.g., 'https://my-resource.services.ai.azure.com')
 *
 * Authentication (one of the following):
 * - ANTHROPIC_FOUNDRY_API_KEY: Your Microsoft Foundry API key (if using API key auth)
 * - Azure AD authentication: If no API key is provided, uses DefaultAzureCredential
 */
```

**端点构建规则**：
- 优先使用 `ANTHROPIC_FOUNDRY_BASE_URL`（直接提供完整 URL）
- 否则使用 `ANTHROPIC_FOUNDRY_RESOURCE` 构造：`https://{resource}.services.ai.azure.com/anthropic/v1/messages`
- 两者都未设置时，依赖 SDK 默认值（或报错）

## §2 Go 侧 anthropic client 现状评估

### 2.1 当前架构

**文件**：`internal/platform/api/anthropic/client.go`

当前 client 已支持三种 provider 模式：

1. **Anthropic first-party**（默认）：`apiKey`/`authToken` + `baseURL` + `x-api-key`/`authorization` header + `anthropic-version`
2. **Vertex AI**（batch-209）：`vertexEnabled` + `vertexProjectID` + `vertexRegion` + `vertexAuth` (GoogleAuthenticator) + OAuth Bearer token
3. **AWS Bedrock**（batch-210）：`bedrockEnabled` + `bedrockRegion` + `bedrockModelID` + `bedrockAuth` (AWSAuthenticator) + AWS Signature V4

**Stream 方法分支结构**：

```go
if c.vertexEnabled { /* Vertex 分支 */ }
if c.bedrockEnabled { /* Bedrock 分支 */ }
// 默认 Anthropic 分支
```

### 2.2 复用策略

Foundry 遵循 batch-209/210 已建立的 provider 扩展模式：

- **不新建独立包**：直接在 `anthropic` 包内扩展
- **Config 扩展**：添加 `FoundryEnabled`、`FoundryResource`、`FoundryBaseURL`、`FoundryAPIKey`、`FoundrySkipAuth`、`FoundryAuth` 字段
- **Client 扩展**：添加 `foundryEnabled`、`foundryResource`、`foundryBaseURL`、`foundryAuth` 字段
- **Stream 新增分支**：在 Bedrock 分支之后、默认 Anthropic 分支之前插入 Foundry 分支
- **认证接口**：`FoundryAuthenticator` 接口（类似 `AWSAuthenticator`/`GoogleAuthenticator`），方法签名更简洁（只需注入 header，无需复杂签名）

**与 Bedrock 的关键差异**：

| 维度 | Bedrock | Foundry |
|------|---------|---------|
| 认证复杂度 | AWS Signature V4（~160 行自实现）| API Key header 注入（~10 行）|
| 端点构建 | region + model ID 映射 | resource / base URL |
| model ID | 需要映射表（11 个模型）| 直接透传（无映射）|
| 请求格式 | `invoke-with-response-stream` | `/v1/messages`（与 Anthropic 相同）|
| accept header | `application/vnd.amazon.eventstream` | `text/event-stream`（与 Anthropic 相同）|

Foundry 的认证和端点逻辑显著更简单，预计实现工作量小于 Bedrock。

## §3 边界确认与范围锁定

### 3.1 纳入范围

1. **API Key 认证**：`ANTHROPIC_FOUNDRY_API_KEY` → `api-key` header 注入
2. **Skip Auth**：`CLAUDE_CODE_SKIP_FOUNDRY_AUTH` → noop authenticator
3. **端点构建**：
   - `ANTHROPIC_FOUNDRY_RESOURCE` → `https://{resource}.services.ai.azure.com/anthropic/v1/messages`
   - `ANTHROPIC_FOUNDRY_BASE_URL` → 直接使用
4. **model ID 透传**：Foundry 直接使用模型名，无需映射表
5. **Bootstrap 集成**：`ProviderFoundry` 常量 + `DefaultEngineFactory` 分支
6. **Stream 方法扩展**：新增 Foundry 分支（跳过 `anthropic-version` 和 `task-budget`，与 Vertex/Bedrock 一致）

### 3.2 排除范围

1. **Azure AD 认证（DefaultAzureCredential）**：
   - TS 侧通过 `@azure/identity` 实现，支持环境变量、托管标识、Azure CLI 等多种方式
   - Go 侧完整复现需引入 Azure SDK 或自实现 token 获取链，超出"最小骨架"
   - 决策：仅实现 API Key 和 Skip Auth，Azure AD 延后

2. **Azure token 自动刷新**：
   - 与 Azure AD 认证绑定，同步延后

3. **Beta header 差异（`advanced-tool-use` vs `tool-search-tool`）**：
   - TS 侧 line 1175 显示 1P/Foundry 使用 `advanced-tool-use`，Vertex/Bedrock 使用 `tool-search-tool`
   - 当前 Go 侧 engine 已统一处理 beta header，不引入 provider 级差异

4. **跨 provider 负载均衡、Circuit breaker**：
   - backlog 明确继续延后

## §4 设计决策

### 4.1 认证接口设计

```go
// FoundryAuthenticator abstracts the credential source used for Foundry authentication.
type FoundryAuthenticator interface {
    // Authenticate adds the Foundry api-key header to the given HTTP request.
    Authenticate(req *http.Request) error
}
```

实现：
- `apiKeyFoundryAuthenticator`：从环境变量或配置读取 API Key，注入 `api-key` header
- `noopFoundryAuthenticator`：空实现（Skip Auth）
- `newFoundryAuthenticator(skipAuth bool)` 工厂：Skip → API Key env → error

### 4.2 端点构建设计

```go
// resolveFoundryBaseURL returns the full base URL for Foundry requests.
// Priority: ANTHROPIC_FOUNDRY_BASE_URL > ANTHROPIC_FOUNDRY_RESOURCE > error
func resolveFoundryBaseURL(resource, baseURL string) (string, error)
```

### 4.3 Stream 分支位置

在现有 Stream 方法的 Bedrock 分支之后、Anthropic 默认分支之前插入 Foundry 分支：

```go
if c.vertexEnabled { /* ... */ }
if c.bedrockEnabled { /* ... */ }
if c.foundryEnabled { /* NEW: Foundry 分支 */ }
// 默认 Anthropic 分支
```

Foundry 分支行为：
- 使用 `resolveFoundryBaseURL` 获取端点 + `/v1/messages`
- 使用 `foundryAuth.Authenticate` 注入认证头
- `accept: text/event-stream`（与 Anthropic 相同）
- **不发送** `anthropic-version` header（与 Vertex/Bedrock 一致）
- **不发送** `task-budget`（与 Vertex/Bedrock 一致，`isFirstParty` 为 false）
