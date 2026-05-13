#!/bin/bash
# TUI 启动脚本：单命令同时启动 Go 后端 + TUI 前端
# Go 后台运行，TUI 独占终端（通过 script 分配 PTY）
# 自动使用临时 session DB，避免迁移冲突

set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
TUI_DIR="$DIR/tui"

# 编译（如果二进制不存在或源码更新）
cd "$DIR"
go build -o /tmp/cc-tui ./cmd/cc/. 2>/dev/null || {
  echo "编译失败" >&2
  exit 1
}

# 使用临时 DB 避免迁移冲突
SESSION_DB="/tmp/cc-tui-sessions.db"
rm -f "$SESSION_DB"

# 启动 Go 后端（后台），捕获端口
CLAUDE_CODE_SESSION_DB_PATH="$SESSION_DB" /tmp/cc-tui --tui 2>/tmp/cc-tui-port.log &
CC_PID=$!

# 等待端口输出（日志包含 ANSI 转义码，需 strip 后解析）
PORT=""
for i in $(seq 1 10); do
  PORT=$(sed 's/\x1b\[[0-9;]*m//g' /tmp/cc-tui-port.log 2>/dev/null | grep -Eo 'port=[0-9]+' | tail -1 | cut -d= -f2)
  if [ -n "$PORT" ]; then
    break
  fi
  sleep 1
done

if [ -z "$PORT" ]; then
  echo "错误：无法获取 WebSocket 端口" >&2
  kill $CC_PID 2>/dev/null
  exit 1
fi

echo "Go 后端已启动 (PID: $CC_PID)"
echo "WebSocket 端口: $PORT"
echo "Go 日志: tail -f /tmp/cc-tui-port.log"

# 显示最近几条 Go 日志（不含 ANIS 转义码）
echo ""
echo "--- 最近 Go 日志 ---"
sed 's/\x1b\[[0-9;]*m//g' /tmp/cc-tui-port.log | grep -v '^[[:space:]]*$' | tail -5
echo "---"
echo ""

# 使用 script 创建 PTY，让 TUI 拥有独立终端
script -q /dev/null sh -c "cd '$TUI_DIR' && bun run src/index.tsx --port $PORT"

# TUI 退出后，清理 Go 进程
kill $CC_PID 2>/dev/null
wait $CC_PID 2>/dev/null || true
echo ""
echo "TUI 已退出"
echo "Go 日志: tail -f /tmp/cc-tui-port.log"
