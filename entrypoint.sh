#!/bin/sh
set -e

# 以 root 启动，修复密钥文件权限后切换到 nobody 运行
if [ "$(id -u)" = "0" ]; then
  if [ -d /app/keys ]; then
    find /app/keys -type d -exec chmod 755 {} + 2>/dev/null || true
    find /app/keys -name '*.pem' -exec chmod 644 {} + 2>/dev/null || true
  fi
  exec su-exec nobody "$0" "$@"
fi

exec ./server
