#!/bin/bash
# 一键重新构建和重启服务（本地开发 / 服务器更新）
set -e

cd "$(dirname "$0")/.."

# 如果在 deploy 目录下运行，使用 deploy 的 docker-compose
if [ -f "deploy/docker-compose.yml" ] && [ -f ".env" ]; then
    cd deploy
fi

echo "================================"
echo "  一键重新部署"
echo "================================"
echo ""

# 检查 .env
if [ ! -f .env ]; then
    echo "[错误] 未找到 .env 文件"
    exit 1
fi

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "[错误] 未安装 Docker"
    exit 1
fi

# 判断是源码部署还是镜像部署
if [ -f "Dockerfile" ]; then
    echo "[模式] 源码构建"
    echo ""
    echo "[1/3] 重新构建镜像..."
    docker-compose build
    echo ""
    echo "[2/3] 重启服务..."
    docker-compose up -d
else
    echo "[模式] 镜像部署"
    echo ""
    if [ -d "images" ]; then
        echo "[1/3] 导入镜像..."
        for img in images/*.tar.gz; do
            name=$(basename "$img" .tar.gz)
            echo "  导入 $name ..."
            docker load -i "$img" > /dev/null 2>&1
        done
    fi
    echo ""
    echo "[2/3] 重启服务..."
    docker-compose down 2>/dev/null || true
    docker-compose up -d
fi

# 等待健康检查
echo ""
echo "[3/3] 等待服务启动..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:3010/health > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

echo ""
echo "================================"
echo "  部署完成"
echo "================================"
echo ""
docker-compose ps
echo ""
echo "健康检查:"
curl -sf http://localhost:3010/health 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "  后端未就绪，请稍后访问"
echo ""
