#!/bin/bash
# 打包部署包：构建镜像 → 导出 → 复制配置 → 生成可部署目录
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(date +%Y%m%d)}"
OUTPUT_DIR="${PROJECT_ROOT}/dist/deploy-${VERSION}"

echo "================================"
echo "  打包部署包 v${VERSION}"
echo "================================"
echo ""

# 1. 构建镜像
echo "[1/4] 构建 Docker 镜像..."
cd "$PROJECT_ROOT"
docker-compose build --no-cache

# 2. 导出镜像
echo "[2/4] 导出镜像..."
mkdir -p "${OUTPUT_DIR}/images"
docker save wwlocal-wework-backend:latest | gzip > "${OUTPUT_DIR}/images/backend.tar.gz"
docker save wwlocal-wework-frontend:latest | gzip > "${OUTPUT_DIR}/images/frontend.tar.gz"
docker save mysql:8.0 | gzip > "${OUTPUT_DIR}/images/mysql.tar.gz"
echo "  导出完成: $(ls -lh ${OUTPUT_DIR}/images/ | awk '{print $5, $9}' | tail -n +2)"

# 3. 复制配置文件
echo "[3/4] 复制配置文件..."
cp "${PROJECT_ROOT}/deploy/docker-compose.yml" "${OUTPUT_DIR}/docker-compose.yml"
cp "${PROJECT_ROOT}/deploy/.env" "${OUTPUT_DIR}/.env"
cp "${PROJECT_ROOT}/deploy/deploy.sh" "${OUTPUT_DIR}/deploy.sh"
chmod +x "${OUTPUT_DIR}/deploy.sh"
mkdir -p "${OUTPUT_DIR}/mysql"
cp "${PROJECT_ROOT}/mysql/my.cnf" "${OUTPUT_DIR}/mysql/my.cnf"

# 复制密钥（如果存在）
if [ -d "${PROJECT_ROOT}/keys/v1" ]; then
    mkdir -p "${OUTPUT_DIR}/keys/v1"
    cp "${PROJECT_ROOT}/keys/v1/"*.pem "${OUTPUT_DIR}/keys/v1/" 2>/dev/null || true
fi
if [ -f "${PROJECT_ROOT}/keys/rsa_private_key.pem" ]; then
    cp "${PROJECT_ROOT}/keys/"*.pem "${OUTPUT_DIR}/keys/" 2>/dev/null || true
fi

# 4. 生成部署说明
echo "[4/4] 生成部署说明..."
cat > "${OUTPUT_DIR}/README.txt" << 'EOF'
政务微信数据查询平台 - 部署包

部署步骤：
  1. 编辑 .env 文件，填入实际配置（必填项不能为空）
  2. 将 RSA 私钥放入 keys/v1/ 目录（如已有可跳过）
  3. 执行: bash deploy.sh

管理命令：
  查看状态: docker-compose ps
  查看日志: docker-compose logs -f backend
  停止服务: docker-compose down
  重启服务: docker-compose restart
  更新部署: bash redeploy.sh
EOF

# 复制一键重启脚本
cp "${PROJECT_ROOT}/scripts/redeploy.sh" "${OUTPUT_DIR}/redeploy.sh"
chmod +x "${OUTPUT_DIR}/redeploy.sh"

echo ""
echo "================================"
echo "  打包完成"
echo "================================"
echo ""
echo "部署包目录: ${OUTPUT_DIR}"
echo "文件大小:   $(du -sh ${OUTPUT_DIR} | awk '{print $1}')"
echo ""
echo "部署步骤:"
echo "  1. 将整个目录上传到服务器"
echo "  2. 编辑 .env 填入配置"
echo "  3. 执行 bash deploy.sh"
echo ""
