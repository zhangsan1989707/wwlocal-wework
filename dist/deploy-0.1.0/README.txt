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
