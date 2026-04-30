# batch-208 阶段 1 源码分析

## §1 TS 侧 `${user_config.X}` 替换源码分析

### 关键文件

- `src/utils/plugins/pluginOptionsStorage.ts` — 替换引擎实现
- `src/utils/plugins/loadPluginCommands.ts` — 替换调用点（skill content 路径）
- `src/utils/hooks.ts` — 替换调用点（hook command 路径）

### 替换引擎

TS 侧提供两个替换函数，按调用场景区分：

#### `substituteUserConfigVariables(value, userConfig)` — 严格替换

- 用于：MCP/LSP server 配置、hook 命令字符串
- 行为：`/\$\{user_config\.([^}]+)\}/g`，提取 `key`，从 `userConfig[key]` 取值
- 缺失键：**抛 Error**，因为 caller 已在 `validateUserConfig` 之后调用，miss = plugin authoring bug
- 不处理敏感值过滤（caller 场景本身不暴露给 model）

#### `substituteUserConfigInContent(content, options, schema)` — 宽松替换

- 用于：skill/agent 内容（会进入 model prompt）
- 行为：
  - `schema[key]?.sensitive === true` → 替换为 `[sensitive option 'key' not available in skill content]`
  - `options[key] === undefined` → **保留字面量** `${user_config.X}`（不抛错，匹配 env var 未设置行为）
  - 否则 → `String(value)`

### 数据来源

- `loadPluginOptions(sourceName)` — 从持久化存储（settings.json + keychain）读取该 plugin 的用户配置值
- 返回类型：`PluginOptionValues = Record<string, string | number | boolean>`

### 替换时机与顺序（`loadPluginCommands.ts` 中）

在 `getPromptForCommand` 中，替换顺序为：
1. `substituteArguments` — 参数替换
2. `substitutePluginVariables` — 插件变量（${CLAUDE_PLUGIN_ROOT}, ${CLAUDE_PLUGIN_DATA}）
3. **`substituteUserConfigInContent`** — 用户配置（${user_config.X}）
4. `${CLAUDE_SKILL_DIR}` 替换
5. `${CLAUDE_SESSION_ID}` 替换
6. `executeShellCommandsInPrompt` — shell 命令内联执行

### 关键设计决策

- **顺序**：plugin 变量先于 user_config，防止用户输入的字面量 `${CLAUDE_PLUGIN_ROOT}` 被二次解释
- **敏感值**：skill/agent 内容中敏感键不暴露实际值；MCP/LSP/hook 中暴露实际值（同 trust boundary）
- **嵌套键**：当前 TS 实现只支持单层键 `userConfig[key]`，不支持 `user_config.a.b` 嵌套访问

---

## §2 TS 侧 Plugin Command Prompt 执行语义源码分析

### 关键文件

- `src/types/command.ts` — `PromptCommand` 类型定义
- `src/utils/plugins/loadPluginCommands.ts` — `getPromptForCommand` 实现
- `src/utils/processUserInput/processSlashCommand.tsx` — 执行编排

### PromptCommand 类型

```ts
type PromptCommand = {
  type: 'prompt'
  progressMessage: string
  contentLength: number
  argNames?: string[]
  allowedTools?: string[]
  model?: string
  source: SettingSource | 'builtin' | 'mcp' | 'plugin' | 'bundled'
  pluginInfo?: { pluginManifest: PluginManifest; repository: string }
  disableNonInteractive?: boolean
  hooks?: HooksSettings
  skillRoot?: string
  context?: 'inline' | 'fork'
  agent?: string
  effort?: EffortValue
  paths?: string[]
  getPromptForCommand(args: string, context: ToolUseContext): Promise<ContentBlockParam[]>
}
```

### `getPromptForCommand` 实现（plugin 路径）

1. skill mode 时添加 base directory 前缀
2. `substituteArguments(finalContent, args, true, argumentNames)` — 参数注入
3. `substitutePluginVariables` — 插件路径变量
4. `substituteUserConfigInContent` — 用户配置变量
5. `${CLAUDE_SKILL_DIR}` 替换
6. `${CLAUDE_SESSION_ID}` 替换
7. `executeShellCommandsInPrompt` — 内联 shell 命令执行
8. 返回 `[{ type: 'text', text: finalContent }]`

### `getMessagesForPromptSlashCommand` 执行编排

```
1. command.getPromptForCommand(args, context) → ContentBlockParam[] (result)
2. registerSkillHooks (if hooks defined)
3. addInvokedSkill (compaction preservation)
4. mainMessageContent = imageContentBlocks + precedingInputBlocks + result
5. attachmentMessages = getAttachmentMessages(textContent, context, ...)
6. messages = [
     createUserMessage({ content: metadata, uuid }),
     createUserMessage({ content: mainMessageContent, isMeta: true }),
     ...attachmentMessages,
     createAttachmentMessage({ type: 'command_permissions', allowedTools, model })
   ]
7. return { messages, shouldQuery: true, allowedTools, model, effort, command }
```

### 执行语义核心

- **inline 模式**（默认）：skill content 展开到当前 conversation，作为用户消息发送，然后 `shouldQuery: true` 触发 engine 查询
- **fork 模式**：在子 agent 中运行，结果作为用户消息返回，`shouldQuery: false`
- 用户输入 `/command args` 的完整流程：解析 slash command → 找到 PromptCommand → 调用 `getPromptForCommand` → 组装 messages → engine query

---

## §3 Go 侧变量替换引擎与 Command 执行现状评估

### 已有能力（batch-206）

`internal/platform/plugin/variables.go`：

- `SubstitutePluginVariables(value, pluginPath, pluginSource)` — `${CLAUDE_PLUGIN_ROOT}`, `${CLAUDE_PLUGIN_DATA}`
- `SubstituteSkillDir(value, skillDir)` — `${CLAUDE_SKILL_DIR}`
- `SubstituteArguments(content, args, argumentNames, appendIfNoPlaceholder)` — `$ARGUMENTS`, `$ARGUMENTS[n]`, `$n`, `$name`

`internal/platform/plugin/command_adapter.go`：

- `CommandAdapter` 实现 `command.Command` 接口
- `Execute` 返回 `command.Result{Output: content}` — **纯文本输出，不触发 engine query**
- 注释：`"does not trigger an engine query (the full prompt execution semantic is not yet migrated)"`

### 缺口识别

| 缺口 | TS 对应 | 状态 | 计划 |
|------|---------|------|------|
| `${user_config.X}` 替换 | `substituteUserConfigVariables` / `substituteUserConfigInContent` | 缺失 | **batch-208 M2-1** |
| userConfig 来源接入 | `loadPluginOptions(sourceName)` | 缺失 | **batch-208 M2-1** |
| Command prompt 执行引擎（注入 conversation + shouldQuery） | `getMessagesForPromptSlashCommand` | 缺失 | **batch-208 M3-1** |
| CommandAdapter prompt 执行路径 | `getPromptForCommand` + Execute 语义扩展 | 缺失 | **batch-208 M3-2** |
| `${CLAUDE_SESSION_ID}` 替换 | `getSessionId()` | 缺失 | 延后 |
| `executeShellCommandsInPrompt` | shell 内联执行 | 缺失 | 延后（batch-192 已做 Skills Shell Execution，但 plugin prompt 中的 shell 执行未接入） |
| coordinator mode 处理 | coordinator 分支 | 缺失 | 延后 |
| forked 子 agent 执行 | `executeForkedSlashCommand` | 缺失 | 延后 |

### Go 侧 conversation 注入架构分析

当前 `command.Command` 接口：

```go
type Command interface {
    Metadata() Metadata
    Execute(ctx context.Context, args Args) (Result, error)
}

type Result struct {
    Output string
}
```

TS 侧 `PromptCommand` 的核心区别：不是返回 `string`，而是返回一组 `ContentBlockParam[]` + 触发 `shouldQuery`。Go 侧需要扩展这个语义。

可能的扩展方向：
1. **扩展 `command.Result`**：添加 `ShouldQuery bool`、`Messages []Message`、`AllowedTools []string`、`Model string` 等字段
2. **或者**：CommandAdapter 的 Execute 保持返回文本，但在 command handler 层（`services/commands/`）识别 plugin command 并做特殊处理

考虑到 Go 侧 `command.Command` 是 core 接口，应保持最小。更合理的方案是：**扩展 `command.Result` 添加可选字段**，让 handler 层根据结果内容决定行为。

---

## §4 边界确认与范围锁定

### 确认纳入 batch-208

1. **`${user_config.X}` 替换引擎扩展**
   - 新增 `SubstituteUserConfigVariables`（严格模式，用于 MCP/LSP/hook 配置）
   - 新增 `SubstituteUserConfigInContent`（宽松模式，用于 skill/agent/command content）
   - 接入 `InstalledPluginsStore` 作为 user config 数据来源
   - 嵌套键支持：当前 TS 实现只支持单层键，Go 侧先对齐单层键

2. **Plugin Command prompt 模板解析**
   - Go 侧已有 `SubstituteArguments`，功能完整，无需改动
   - 需要确认 Go 侧 `CommandAdapter.Execute` 中的替换顺序与 TS 侧一致

3. **Command Prompt 执行引擎**
   - 扩展 `core/command.Result` 添加 `ShouldQuery bool` 等字段
   - CommandAdapter 实现 `getPromptForCommand` 等价的完整逻辑
   - 在 command handler（`services/commands/` 或 REPL/Engine）中识别 `ShouldQuery=true` 并注入 conversation

### 确认不纳入 batch-208

- `${CLAUDE_SESSION_ID}` 替换（需 session ID 来源，延后）
- `executeShellCommandsInPrompt`（shell 内联执行，延后）
- coordinator mode 处理（延后）
- forked 子 agent 执行（需 AgentRunner 扩展，延后）
- skill hooks 注册（batch-207 已做 hooks 基础设施，但 skill-level hooks 注册未接入，延后）
- attachment 消息提取（`getAttachmentMessages`，延后）
- allowedTools 作为 permission attachment 传递（延后）

### 接口设计决策

**UserConfig 来源接口**：
- 在 `PluginRegistrar` 或 `PluginLoader` 中持有 `*InstalledPluginsStore` 引用（已有）
- 新增 `LoadPluginOptions(pluginSource string) map[string]interface{}` 方法
- `SubstituteUserConfig*` 函数接收 `map[string]interface{}` 作为配置值来源

**command.Result 扩展**：
```go
type Result struct {
    Output       string
    ShouldQuery  bool           // true = 将 Output 作为用户消息注入 conversation 并触发 engine query
    AllowedTools []string       // 仅当 ShouldQuery=true 时有效
}
```

**CommandAdapter 执行路径**：
- 当前 `Execute` 返回纯文本 → 保持为 fallback 路径
- 新增完整执行路径：解析 prompt → 变量替换 → 参数替换 → 返回 `Result{ShouldQuery: true, ...}`
