#!/bin/bash
# 部署脚本：首次部署 或 重新部署
set -e

cd "$(dirname "$0")"

echo "================================"
echo "  政务微信数据查询平台 部署"
echo "================================"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "[错误] 未安装 Docker"
    exit 1
fi
if ! command -v docker-compose &> /dev/null; then
    echo "[错误] 未安装 docker-compose"
    exit 1
fi

# 检查 .env
if [ ! -f .env ]; then
    echo "[错误] 未找到 .env 文件"
    echo "请复制 .env.example 为 .env 并填写配置"
    exit 1
fi

# 检查必填项
source .env
missing=""
[ -z "$DB_PASSWORD" ] && missing="$missing DB_PASSWORD"
[ -z "$WEWORK_CORPID" ] && missing="$missing WEWORK_CORPID"
[ -z "$WEWORK_SECRET" ] && missing="$missing WEWORK_SECRET"
[ -z "$AUTH_PASSWORD" ] && missing="$missing AUTH_PASSWORD"
[ -z "$JWT_SECRET" ] && missing="$missing JWT_SECRET"
if [ -n "$missing" ]; then
    echo "[错误] .env 中以下必填项为空:$missing"
    exit 1
fi

# 停止旧服务
echo "[1/4] 停止旧服务..."
docker-compose down 2>/dev/null || true

# 导入镜像
if [ -d "images" ]; then
    echo "[2/4] 导入 Docker 镜像..."
    for img in images/*.tar.gz; do
        name=$(basename "$img" .tar.gz)
        echo "  导入 $name ..."
        docker load -i "$img" > /dev/null 2>&1
    done
    echo "  镜像导入完成"
else
    echo "[2/4] 跳过镜像导入（无 images 目录）"
fi

# 启动服务
echo "[3/4] 启动服务..."
docker-compose up -d

# 等待健康检查
echo "[4/4] 等待服务启动..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:${BACKEND_PORT:-19010}/health > /dev/null 2>&1; then
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
echo "前端地址: http://$(hostname -I 2>/dev/null | awk '{print $1}' || hostname):${FRONTEND_PORT:-18073}"
echo "后端地址: http://$(hostname -I 2>/dev/null | awk '{print $1}' || hostname):${BACKEND_PORT:-19010}"
echo ""
echo "管理命令:"
echo "  查看状态: docker-compose ps"
echo "  查看日志: docker-compose logs -f backend"
echo "  停止服务: docker-compose down"
echo "  重启服务: docker-compose restart"
echo "  重新部署: bash deploy.sh"
echo ""
