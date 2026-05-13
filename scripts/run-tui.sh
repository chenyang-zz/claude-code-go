#!/bin/bash
# TUI 启动脚本：单命令同时启动 Go 后端 + TUI 前端
# Go 后台运行，TUI 独占终端（通过 script 分配 PTY）

set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
TUI_DIR="$DIR/tui"

# 编译（如果二进制不存在或源码更新）
cd "$DIR"
go build -o /tmp/cc-tui ./cmd/cc/. 2>/dev/null || {
  echo "编译失败" >&2
  exit 1
}

# 启动 Go 后端（后台），捕获端口
/tmp/cc-tui --tui 2>/tmp/cc-tui-port.log &
CC_PID=$!

# 等待端口输出
PORT=""
for i in $(seq 1 10); do
  PORT=$(grep -Eo 'port=[0-9]+' /tmp/cc-tui-port.log 2>/dev/null | tail -1 | cut -d= -f2)
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

# 使用 script 创建 PTY，让 TUI 拥有独立终端
script -q /dev/null sh -c "cd '$TUI_DIR' && bun run src/index.tsx --port $PORT"

# TUI 退出后，清理 Go 进程
kill $CC_PID 2>/dev/null
wait $CC_PID 2>/dev/null
echo "TUI 已退出"
