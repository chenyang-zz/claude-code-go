# claude-code-go

`claude-code-go` 是针对当前恢复版 `@anthropic-ai/claude-code` 的 Go 迁移骨架。

当前目录只提供顶级架构、核心接口和最小可编译入口，不尝试直接复刻原 TS/Bun/Ink 实现。

## 目标

- 先建立稳定的分层架构
- 再逐步迁移 conversation engine、tool runtime、permission、MCP、plugin
- 保持 UI 与核心逻辑解耦

## 当前状态

- 可编译的最小入口已建立
- 核心接口已建模
- 目录结构已按长期演进拆层

## 运行

```bash
go run ./cmd/cc
```
