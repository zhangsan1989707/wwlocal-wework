# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

政务微信数据查询平台 - 从政务微信开放接口获取加密业务日志，支持RSA+AES双引擎解密，数据按FeatureID+月分表存储。

## 常用命令

### 后端 (Go)
```bash
# 构建
go build ./cmd/server/...

# 运行
go run cmd/server/main.go

# 依赖更新
go mod tidy
```

### 前端 (Vue)
```bash
cd web

# 安装依赖
npm install

# 开发模式
npm run dev

# 生产构建
npm run build
```

### Docker
```bash
# 启动所有服务 (开发环境)
docker-compose up -d --build

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs backend
docker-compose logs frontend
docker-compose logs mysql

# 停止服务
docker-compose down
```

## 架构

### 分层结构
```
cmd/server/main.go          # 入口，依赖注入
config/config.go            # 配置加载 (支持DB_HOST等环境变量)
internal/
  ├── handler/              # HTTP处理层
  ├── service/             # 业务逻辑层 (wework_service, decrypt_service, sync_service, query_service)
  ├── repository/          # 数据访问层 (log_repository, key_repository)
  ├── model/               # 数据模型
  └── crypto/              # 加密解密 (rsa_decrypt, aes_decrypt)
pkg/                        # 公共工具 (httpclient, response)
```

### 核心流程
1. **数据同步**: `syncService.SyncFeature()` → `weworkService.GetLogList()` → `decryptService.Decrypt()` → `logRepository.Save()`
2. **解密流程**: RSA PKCS1v15解密enc_key → AES-128-CBC PKCS7解密切密文
3. **分表策略**: `log_{feature_id}_{YYYYMM}` 格式

### 密钥热切换
- 密钥存储在 `keys/{version}/rsa_private_key.pem`
- 数据库表 `rsa_key_versions` 管理版本和激活状态
- 解密时优先使用激活版本，失败则遍历历史版本

### API路由
- `GET /health` - 健康检查
- `POST /api/v1/logs/query` - 日志查询
- `POST /api/v1/logs/sync` - 触发同步
- `GET/POST /api/v1/keys` - 密钥管理
- `PUT /api/v1/keys/activate` - 激活密钥

## 配置

配置文件: `config.yaml`
- 数据库: `${DB_HOST:-mysql}` 等环境变量覆盖
- 政务微信: `wework.base_url` = `https://uat.rztcd.cn:89`

## 技术栈
- **后端**: Go 1.21 + Echo v4 + GORM + MySQL
- **前端**: Vue 3 + TypeScript + Element Plus + Vite
- **容器**: Docker + docker-compose