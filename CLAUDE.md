# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

政务微信数据查询平台 - 从政务微信开放接口获取加密业务日志，RSA+AES 双层解密后存储到 MySQL，前端提供查询/同步/密钥管理功能。

## 常用命令

### 后端 (Go)
```bash
go build ./cmd/server/...    # 构建
go run cmd/server/main.go    # 运行
go mod tidy                  # 依赖更新
```

### 前端 (Vue)
```bash
cd web
npm install                  # 安装依赖
npm run dev                  # 开发模式 (端口 5173)
npm run build                # 生产构建 (含类型检查)
```

### Docker
```bash
docker-compose up -d --build   # 启动所有服务
docker-compose ps              # 查看状态
docker-compose logs backend    # 查看后端日志
docker-compose down            # 停止服务
```

端口映射: MySQL `3307`, 后端 `3010`, 前端 `5173`

## 架构

### 依赖注入

`cmd/server/main.go` 是唯一的组合根，手动构造注入，无框架：
```
Config → GORM DB → Repositories → Services → Handlers → Router → Echo
```

### 分层结构

```
cmd/server/main.go              # 入口，组合根
config/config.go                # 配置加载，DB_* 环境变量覆盖
internal/
  ├── handler/                  # HTTP 处理层，定义本地 Request struct
  ├── service/                  # 业务逻辑层
  ├── repository/               # 数据访问层
  ├── model/                    # 数据模型（部分未使用）
  ├── router/                   # Echo 路由注册
  └── crypto/                   # RSA/AES 解密实现
pkg/
  ├── response/                 # 统一 JSON 响应封装
  └── httpclient/               # 通用 HTTP 客户端（未使用，死代码）
```

### 核心流程

1. **数据同步**: `SyncService.SyncFeature()` → `WeWorkService.GetLogList()` → `DecryptService.Decrypt()` → `LogRepository.Save()`
2. **解密管道**: RSA PKCS1v15 解密 `encrypt_key`(base64) → 得到 16 字节 AES 密钥 → AES-128-CBC 解密 `encrypt_data`(base64, IV = 密文前 16 字节)
3. **分表策略**: 表名 `log_{feature_id}_{YYYYMM}`，由 `LogRepository` 动态创建

### 密钥热切换

- PEM 文件存储在 `keys/{version}/rsa_private_key.pem`
- `rsa_key_versions` 表管理版本和激活状态
- 解密时优先使用激活版本，失败则遍历历史版本回退
- `RSADecryptor` 实例按版本缓存在内存中

### 前端架构

- 无路由库，`App.vue` 中通过 `v-if` 切换三个视图组件
- 无状态管理，各组件独立用 `ref`/`reactive` 管理状态
- Element Plus 组件通过 `unplugin-auto-import` 自动导入
- Axios 实例在 `src/api/index.ts`，`baseURL: '/api/v1'`，dev 时代理到 `localhost:8080`

### API 路由

| Method | Path | Handler |
|--------|------|---------|
| GET | `/health` | `HealthHandler.Check` |
| POST | `/api/v1/logs/query` | `LogHandler.Query` |
| GET | `/api/v1/logs/features` | `LogHandler.GetFeatures` |
| GET | `/api/v1/logs/time-range` | `LogHandler.GetTimeRange` |
| POST | `/api/v1/logs/sync` | `SyncHandler.Sync` (goroutine 异步执行) |
| GET | `/api/v1/logs/sync/status` | `SyncHandler.Status` |
| GET/POST | `/api/v1/keys` | `KeyHandler.List` / `KeyHandler.Add` |
| PUT | `/api/v1/keys/activate` | `KeyHandler.Activate` |

## 已知问题

- `KeyRepository.SetActive()` 只设置目标为 true，不取消其他版本的激活状态
- `QueryService.Query()` 内存分页：从 DB 取最多 10000 条，在 Go 中过滤后切片，不适合大数据量
- Feature ID 列表在 `SyncService` 和 `QueryService` 中重复硬编码（90000031-90000066）
- `LogRepository` 使用原生 SQL，其余 Repository 使用 GORM 查询构建器
- `KeyHandler` 直接依赖 Repository，跳过了 Service 层
- 错误响应统一返回 HTTP 200，通过 JSON `code` 字段区分

## 技术栈

- **后端**: Go 1.21 + Echo v4 + GORM + MySQL 8.0
- **前端**: Vue 3.5 + TypeScript + Element Plus + Vite 8
- **容器**: Docker + docker-compose
