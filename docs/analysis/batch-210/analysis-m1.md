# batch-210 分析文档：Bedrock Provider 最小骨架

## §1 TS 侧 Bedrock Provider 源码分析

### 1.1 Provider 选择与入口

`src/utils/model/providers.ts` 定义了四种 provider：

```ts
export type APIProvider = 'firstParty' | 'bedrock' | 'vertex' | 'foundry'
```

Bedrock 通过 `CLAUDE_CODE_USE_BEDROCK` 环境变量启用：

```ts
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

### 1.2 Bedrock SDK 使用方式

`src/services/api/client.ts:153-189` 是 Bedrock client 构建的核心逻辑：

```ts
const { AnthropicBedrock } = await import('@anthropic-ai/bedrock-sdk')
const awsRegion =
  model === getSmallFastModel() && process.env.ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION
    ? process.env.ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION
    : getAWSRegion()

const bedrockArgs = {
  ...ARGS,  // defaultHeaders, maxRetries, timeout, fetchOptions
  awsRegion,
  ...(isEnvTruthy(process.env.CLAUDE_CODE_SKIP_BEDROCK_AUTH) && { skipAuth: true }),
  ...(isDebugToStdErr() && { logger: createStderrLogger() }),
}

// Bearer token 认证（API key 模式）
if (process.env.AWS_BEARER_TOKEN_BEDROCK) {
  bedrockArgs.skipAuth = true
  bedrockArgs.defaultHeaders = {
    ...bedrockArgs.defaultHeaders,
    Authorization: `Bearer ${process.env.AWS_BEARER_TOKEN_BEDROCK}`,
  }
} else if (!isEnvTruthy(process.env.CLAUDE_CODE_SKIP_BEDROCK_AUTH)) {
  // AWS Signature V4 认证（默认）
  const cachedCredentials = await refreshAndGetAwsCredentials()
  if (cachedCredentials) {
    bedrockArgs.awsAccessKey = cachedCredentials.accessKeyId
    bedrockArgs.awsSecretKey = cachedCredentials.secretAccessKey
    bedrockArgs.awsSessionToken = cachedCredentials.sessionToken
  }
}
return new AnthropicBedrock(bedrockArgs) as unknown as Anthropic
```

关键结论：
- `@anthropic-ai/bedrock-sdk` 封装了 AWS Bedrock Runtime API，对外暴露与 Anthropic SDK 相同的接口
- 内部自动处理 AWS Signature V4 签名
- 支持三种认证模式：AWS 密钥对（默认）、Bearer Token、Skip Auth

### 1.3 区域选择逻辑

`src/utils/envUtils.ts:96-97`：

```ts
export function getAWSRegion(): string {
  return process.env.AWS_REGION || process.env.AWS_DEFAULT_REGION || 'us-east-1'
}
```

小快模型（Haiku）可单独覆盖：`ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION`

### 1.4 模型 ID 映射

`src/utils/model/configs.ts` 定义了每个模型的四 provider ID 映射：

| 模型 | firstParty | bedrock | vertex | foundry |
|------|------------|---------|--------|---------|
| sonnet 4-5 | claude-sonnet-4-5-20250929 | us.anthropic.claude-sonnet-4-5-20250929-v1:0 | claude-sonnet-4-5@20250929 | claude-sonnet-4-5 |
| opus 4-6 | claude-opus-4-6 | us.anthropic.claude-opus-4-6-v1 | claude-opus-4-6 | claude-opus-4-6 |

Bedrock ID 格式规则：
- Cross-region inference：`{prefix}.anthropic.claude-{model}-{version}-v{N}:0`
- Foundation model：`anthropic.claude-{model}-{version}-v{N}:0`
- 最新模型（opus46/sonnet46）：`us.anthropic.claude-{model}-v1`

### 1.5 Bedrock 特有功能（超出本批范围）

`src/utils/model/bedrock.ts` 包含以下功能（本批不实现）：
- `getBedrockInferenceProfiles()` - 列出 inference profiles（需 `@aws-sdk/client-bedrock`）
- `createBedrockClient()` / `createBedrockRuntimeClient()` - AWS SDK 原生客户端
- `getBedrockRegionPrefix()` / `applyBedrockRegionPrefix()` - Cross-region prefix 处理
- `extractModelIdFromArn()` - ARN 解析
- `getInferenceProfileBackingModel()` - Inference profile 背后的模型查询

### 1.6 消息格式

与 Anthropic Messages API 完全相同。`@anthropic-ai/bedrock-sdk` 内部将 Anthropic 格式转换为 Bedrock InvokeModel/Converse API 格式。

### 1.7 环境变量汇总

| 环境变量 | 用途 |
|----------|------|
| `CLAUDE_CODE_USE_BEDROCK` | 启用 Bedrock provider |
| `AWS_REGION` / `AWS_DEFAULT_REGION` | AWS 区域 |
| `ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION` | 小快模型区域覆盖 |
| `CLAUDE_CODE_SKIP_BEDROCK_AUTH` | 跳过认证 |
| `AWS_BEARER_TOKEN_BEDROCK` | Bearer token 认证 |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_SESSION_TOKEN` | AWS 密钥对（通过 SDK 默认链读取）|
| `ANTHROPIC_BEDROCK_BASE_URL` | 端点覆盖（测试/代理）|

## §2 Go 侧 anthropic client 现状评估

### 2.1 batch-209 Vertex 扩展回顾

`internal/platform/api/anthropic/client.go` 当前状态：

**Config 扩展（5 个 Vertex 字段）：**
- `VertexEnabled bool` - 启用 Vertex 模式
- `VertexProjectID string` - GCP project ID
- `VertexRegion string` - GCP region
- `VertexSkipAuth bool` - 跳过认证（测试）
- `VertexAuth GoogleAuthenticator` - 可注入的认证器（测试 mock）

**Client 扩展（4 个 Vertex 字段）：**
- `vertexEnabled bool`
- `vertexProjectID string`
- `vertexRegion string`
- `vertexAuth GoogleAuthenticator`

**NewClient 初始化：**
- 当 `cfg.VertexEnabled` 为 true 时，初始化 Vertex 字段
- project ID fallback：`cfg.VertexProjectID` → `getVertexProjectID()`（环境变量）
- region fallback：`cfg.VertexRegion` → `resolveVertexRegion("")`
- auth fallback：`cfg.VertexAuth` → `newGoogleAuthenticator(cfg.VertexSkipAuth)`

**Stream 方法 Vertex 分支：**
- 端点：`buildVertexEndpointWithHost(host, region, projectID, model)`
- 认证：`Bearer {token}`（OAuth2 token）
- 跳过：`anthropic-version` header 和 `task-budget` body 字段

### 2.2 复用策略确认

Bedrock 应采用与 Vertex 完全相同的扩展模式：

1. **不新建独立 bedrock 包** - 复用 `anthropic.Client` 的消息协议、mapper、`stream` 解析
2. **扩展 `anthropic.Config`** - 新增 Bedrock 相关字段
3. **扩展 `anthropic.Client`** - 新增 Bedrock 相关字段
4. **`Stream` 方法添加 Bedrock 分支** - 端点切换 + 认证头注入
5. **认证抽象为接口** - 可 mock 的 AWS 签名器

### 2.3 关键差异对比

| 维度 | Vertex | Bedrock |
|------|--------|---------|
| SDK | `@anthropic-ai/vertex-sdk` | `@anthropic-ai/bedrock-sdk` |
| 认证 | OAuth2 Bearer token | AWS Signature V4 / Bearer token |
| 端点格式 | `aiplatform.googleapis.com` | `bedrock-runtime.{region}.amazonaws.com` |
| 请求路径 | `:streamRawPredict` | `/model/{modelId}/invoke-with-response-stream` |
| 模型 ID | `claude-sonnet-4-5@20250929` | `us.anthropic.claude-sonnet-4-5-20250929-v1:0` |
| 区域选择 | `CLOUD_ML_REGION` / `us-east5` | `AWS_REGION` / `AWS_DEFAULT_REGION` / `us-east-1` |
| 跳过认证 | `CLAUDE_CODE_SKIP_VERTEX_AUTH` | `CLAUDE_CODE_SKIP_BEDROCK_AUTH` |

## §3 边界确认与范围锁定

### 3.1 纳入本批的能力

1. **AWS Signature V4 签名基础设施**
   - 接口抽象：`AWSAuthenticator`（类似 `GoogleAuthenticator`）
   - 默认实现：从 `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_SESSION_TOKEN` 环境变量读取
   - 测试实现：`noopAWSAuthenticator`（skip auth）
   - 签名算法：标准 AWS Signature V4（HTTP 请求签名）

2. **Bedrock 端点构建**
   - Region 选择：`AWS_REGION` → `AWS_DEFAULT_REGION` → `us-east-1`
   - 端点 URL：`https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke-with-response-stream`
   - BaseURL 覆盖：支持 `ANTHROPIC_BEDROCK_BASE_URL`（测试/代理）

3. **Model ID 映射**
   - 将内部模型名（如 `claude-sonnet-4-5`）映射为 Bedrock model ID
   - 映射表从 `configs.ts` 提取
   - 支持 cross-region prefix（`us.` 等）

4. **`anthropic.Client` Bedrock 路径扩展**
   - Config 新增：`BedrockEnabled`, `BedrockRegion`, `BedrockModelID`, `BedrockSkipAuth`, `BedrockAuth`
   - Client 新增对应字段
   - `Stream` 方法添加 Bedrock 分支：端点切换、AWS Signature V4 签名头注入
   - 跳过 `anthropic-version` header（Bedrock 不需要）
   - 跳过 `task-budget` body 字段（Bedrock 不支持）

5. **Bootstrap 集成**
   - `core/config/provider.go` 新增 `ProviderBedrock = "bedrock"`
   - `core/config/config.go` 新增 Bedrock 配置字段
   - `app.go` `DefaultEngineFactory` 新增 Bedrock 分支

6. **单元测试**
   - `bedrock_auth_test.go` - 签名算法测试、mock 测试
   - `bedrock_endpoint_test.go` - 端点 URL 构建测试、region 选择边界
   - Client 扩展测试 - 配置切换、认证头验证

### 3.2 排除本批的能力

1. **AWS 凭据自动刷新定时器** - TS 侧 `refreshAndGetAwsCredentials()` 使用 `memoizeWithTTLAsync` 缓存，本批使用环境变量直读
2. **Bedrock guardrails / trace** - 高级参数，最小骨架不需要
3. **Inference profiles 发现** - `getBedrockInferenceProfiles()` 需要 AWS SDK 客户端
4. **Cross-region prefix 动态应用** - `applyBedrockRegionPrefix()` 运行时逻辑
5. **Foundation model 检测** - `isFoundationModel()`
6. **ARN 解析** - `extractModelIdFromArn()`
7. **小快模型区域覆盖** - `ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION`

### 3.3 设计决策

1. **AWS Signature V4 自实现 vs 引入 AWS SDK**
   - 决策：自实现轻量签名算法（约 200 行）
   - 理由：batch-209 选择了轻量实现（不用 `golang.org/x/oauth2/google`），保持一致性；避免引入大依赖

2. **请求路径选择**
   - 决策：使用 Bedrock InvokeModel 流式端点 `/model/{modelId}/invoke-with-response-stream`
   - 理由：`@anthropic-ai/bedrock-sdk` 底层使用此端点，消息体与 Anthropic Messages API 格式相同

3. **模型映射方式**
   - 决策：在 `anthropic/bedrock_endpoint.go` 中维护映射函数 `toBedrockModelID(model string)`
   - 理由：与 Vertex 的 `resolveVertexRegion` 模式一致；映射表从 `configs.ts` 提取

4. **认证接口设计**
   - 决策：模仿 `GoogleAuthenticator` 接口设计 `AWSAuthenticator`
   - 理由：保持与 Vertex 的架构对称性；便于测试 mock

5. **Bearer token 支持**
   - 决策：纳入（`AWS_BEARER_TOKEN_BEDROCK` 环境变量）
   - 理由：TS 侧支持此模式，实现简单（只需设置 `Authorization: Bearer ...` header）
