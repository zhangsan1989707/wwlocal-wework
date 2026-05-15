#!/bin/bash
# 本地一键重新构建和启动（开发环境）
set -e

cd "$(dirname "$0")"

echo "================================"
echo "  一键重新构建和启动"
echo "================================"
echo ""

# 构建并启动
docker-compose up -d --build

echo ""
echo "================================"
echo "  启动完成"
echo "================================"
echo ""
docker-compose ps
echo ""
curl -sf http://localhost:3010/health 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "后端启动中..."
echo ""
