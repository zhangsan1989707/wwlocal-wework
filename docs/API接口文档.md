# API 接口文档

## 通用说明

### 基础 URL

- 生产环境: `http://<host>:19010`
- 开发环境: `http://localhost:8080`

### 认证方式

登录接口无需认证。其余 `/api/v1/*` 接口均需要在请求头中携带 JWT Token：

```
Authorization: Bearer <token>
```

Token 通过 `/api/v1/auth/login` 获取，有效期 24 小时。

### 统一响应格式

所有接口返回统一 JSON 结构：

```json
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}
```

- `code`: 状态码，`0` 表示成功，非 `0` 表示失败（常见：`400` 参数错误、`401` 未认证、`409` 冲突、`500` 服务器错误）
- `msg`: 状态描述
- `data`: 业务数据（错误时可能不存在）

### 分页参数

列表类接口通用分页参数（Query String）：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数（部分接口上限 100） |

---

## 1. 健康检查

### GET /health

无需认证。检查数据库连通性和密钥目录可读性。

**响应示例（正常，HTTP 200）：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "status": "ok",
    "checks": {
      "db": "ok",
      "keys": "ok"
    }
  }
}
```

**响应示例（异常，HTTP 503）：**

```json
{
  "status": "unhealthy",
  "checks": {
    "db": "ok",
    "keys": "error: stat /app/keys: no such file or directory"
  }
}
```

---

## 2. 认证

### POST /api/v1/auth/login

用户登录，获取 JWT Token。内置 IP 限流：1 分钟内同一 IP 最多 5 次失败尝试，之后锁定 1 分钟。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**请求示例：**

```json
{
  "username": "admin",
  "password": "your_password"
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "username": "admin"
  }
}
```

**错误响应：**

| code | msg | 含义 |
|------|-----|------|
| 400 | invalid request body | 请求体格式错误 |
| 401 | invalid username or password | 用户名或密码错误 |
| 429 | 登录尝试过于频繁，请稍后再试 | IP 限流 |

---

## 3. 日志查询

### POST /api/v1/logs/query

查询业务日志数据。支持按数据类型、时间范围、手机号、自定义条件过滤，支持分页和实时查询模式。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| feature_ids | int[] | 是 | 数据类型 ID 列表 |
| start_time | int64 | 是 | 开始时间（Unix 时间戳，秒） |
| end_time | int64 | 是 | 结束时间（Unix 时间戳，秒） |
| conditions | map | 否 | 自定义过滤条件（key 为字段路径，value 为匹配值） |
| mobile | string | 否 | 手机号过滤（匹配 openid） |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20 |
| realtime | bool | 否 | 是否实时查询（直连政务微信 API，跳过本地存储） |

**请求示例：**

```json
{
  "feature_ids": [90000031, 90000032],
  "start_time": 1700000000,
  "end_time": 1700086400,
  "mobile": "13800138000",
  "page": 1,
  "page_size": 50
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 156,
    "page": 1,
    "page_size": 50,
    "data": [
      {
        "feature_id": 90000031,
        "log_time": 1700012345,
        "parsed_json": {
          "openid": "13800138000",
          "action": "login",
          "device": "iPhone"
        },
        "enc_data_hash": "d41d8cd98f00b204e9800998ecf8427e"
      }
    ]
  }
}
```

---

### GET /api/v1/logs/features

获取已知的数据类型列表。

> 注意：此接口标记为 deprecated，建议使用 `GET /api/v1/sync-features` 替代。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    { "id": 90000031, "name": "会话存档" },
    { "id": 90000032, "name": "日程" }
  ]
}
```

---

### GET /api/v1/logs/time-range

获取默认查询时间范围（近 7 天）。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "start_time": 1699449600,
    "end_time": 1699535999,
    "now": 1699536000
  }
}
```

---

### GET /api/v1/logs/field-paths

获取各数据类型的可查询字段路径列表，用于前端构建条件过滤器。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "90000031": ["openid", "action", "device"],
    "90000032": ["openid", "title", "start_time"]
  }
}
```

---

## 4. 数据同步

### POST /api/v1/logs/sync

触发日志数据同步任务。同步以 goroutine 异步执行，调用后立即返回。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| feature_ids | int[] | 否 | 要同步的数据类型 ID 列表 |
| start_time | int64 | 否 | 开始时间（Unix 时间戳） |
| end_time | int64 | 否 | 结束时间（Unix 时间戳） |
| sync_all | bool | 否 | 是否同步所有数据类型 |

`sync_all` 为 `true` 时忽略 `feature_ids`；未指定 `sync_all` 且 `feature_ids` 为空则返回空操作。

**请求示例：**

```json
{
  "sync_all": true,
  "start_time": 1700000000,
  "end_time": 1700086400
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "sync started",
    "running": true
  }
}
```

**错误响应：**

| code | msg |
|------|-----|
| 409 | sync already in progress |

---

### POST /api/v1/logs/sync/cancel

取消正在进行的同步任务。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "sync cancellation requested"
  }
}
```

**错误响应：**

| code | msg |
|------|-----|
| 400 | no sync in progress |

---

### GET /api/v1/logs/sync/status

获取当前同步状态。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": true,
    "progress": 65,
    "current_feature": 90000031,
    "total_features": 5,
    "processed": 3
  }
}
```

---

## 5. 密钥管理

### GET /api/v1/keys

列出所有 RSA 密钥版本。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 1,
      "version": "v1",
      "private_key_path": "keys/v1/rsa_private_key.pem",
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z",
      "activated_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "version": "v2",
      "private_key_path": "keys/v2/rsa_private_key.pem",
      "is_active": false,
      "created_at": "2024-02-01T00:00:00Z"
    }
  ]
}
```

---

### POST /api/v1/keys

添加新的 RSA 密钥版本。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| version | string | 是 | 版本标识（如 "v2"） |
| private_key_pem | string | 是 | PEM 格式的 RSA 私钥内容（完整文本） |

**请求示例：**

```json
{
  "version": "v2",
  "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----"
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 2,
    "version": "v2",
    "private_key_path": "keys/v2/rsa_private_key.pem",
    "is_active": false,
    "created_at": "2024-02-01T00:00:00Z"
  }
}
```

---

### PUT /api/v1/keys/activate

激活指定版本的密钥。仅影响后续解密操作，不会修改历史数据。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| version | string | 是 | 要激活的密钥版本 |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "key activated",
    "version": "v2"
  }
}
```

---

### GET /api/v1/keys/test?version=v1

测试指定密钥版本是否可以正常解密。

**Query 参数：**

| 参数 | 必填 | 说明 |
|------|------|------|
| version | 是 | 要测试的密钥版本 |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "version": "v1",
    "valid": true,
    "decryptable": true
  }
}
```

---

## 6. 定时调度

### POST /api/v1/scheduler/start

启动定时自动同步任务。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_delay | string | 否 | 首次执行延迟（如 "10m"、"30m"、"1h"），最少 1 分钟 |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": true,
    "interval": "1h",
    "next_run": "2024-01-01T02:00:00Z"
  }
}
```

---

### POST /api/v1/scheduler/stop

停止定时同步任务。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": false,
    "interval": "1h"
  }
}
```

---

### GET /api/v1/scheduler/status

获取定时调度器状态。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": true,
    "interval": "1h",
    "last_run": "2024-01-01T01:00:00Z",
    "next_run": "2024-01-01T02:00:00Z"
  }
}
```

---

### POST /api/v1/scheduler/sync

手动触发一次增量同步。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| feature_ids | int[] | 否 | 要同步的数据类型 ID 列表 |
| sync_all | bool | 否 | 同步所有数据类型（默认为 true） |

未指定 `feature_ids` 且未设置 `sync_all` 时，等同于 `sync_all: true`。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "incremental sync started",
    "running": true
  }
}
```

---

### PUT /api/v1/scheduler/interval

修改定时同步的间隔时间。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| interval | string | 是 | 时间间隔，如 "1h"、"30m"、"24h"，最少 "1m" |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": true,
    "interval": "30m"
  }
}
```

---

## 7. 通讯录

### GET /api/v1/contacts

分页查询通讯录联系人列表。

**Query 参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数（上限 100） |
| name | string | - | 按姓名模糊搜索 |
| mobile | string | - | 按手机号精确搜索 |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 350,
    "page": 1,
    "page_size": 20,
    "data": [
      {
        "userid": "zhangsan",
        "name": "张三",
        "mobile": "13800138000",
        "gender": 1,
        "email": "zhangsan@example.com",
        "position": "科长",
        "department": "[1,2]",
        "status": 1,
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-15T00:00:00Z"
      }
    ]
  }
}
```

---

### GET /api/v1/contacts/departments

获取所有部门列表（平铺）。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    { "id": 1, "name": "总公司", "parentid": 0, "order": 0, "type": 0 },
    { "id": 2, "name": "技术部", "parentid": 1, "order": 1, "type": 0 }
  ]
}
```

---

### GET /api/v1/contacts/tree

获取部门树形结构，包含各部门成员数量。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "tree": [
      {
        "id": 1,
        "name": "总公司",
        "parentid": 0,
        "order": 0,
        "type": 0,
        "member_count": 350,
        "children": [
          {
            "id": 2,
            "name": "技术部",
            "parentid": 1,
            "order": 1,
            "type": 0,
            "member_count": 50,
            "children": []
          }
        ]
      }
    ],
    "total": 350
  }
}
```

---

### GET /api/v1/contacts/departments/:id/members

获取指定部门的成员列表（分页）。

**路径参数：**

| 参数 | 说明 |
|------|------|
| id | 部门 ID |

**Query 参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数（上限 100） |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 50,
    "page": 1,
    "page_size": 20,
    "data": [
      {
        "userid": "zhangsan",
        "name": "张三",
        "mobile": "13800138000",
        "position": "科长"
      }
    ]
  }
}
```

---

### GET /api/v1/contacts/:userId

获取单个联系人详情。

**路径参数：**

| 参数 | 说明 |
|------|------|
| userId | 用户 ID |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "userid": "zhangsan",
    "name": "张三",
    "mobile": "13800138000",
    "gender": 1,
    "email": "zhangsan@example.com",
    "position": "科长",
    "department": "[1,2]",
    "status": 1
  }
}
```

**错误响应：**

| code | msg |
|------|-----|
| 404 | contact not found |

---

### POST /api/v1/contacts/names

批量查询联系人姓名（通过 userid）。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_ids | string[] | 是 | 用户 ID 列表（最多 200 个） |

**请求示例：**

```json
{
  "user_ids": ["zhangsan", "lisi", "wangwu"]
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "zhangsan": "张三",
    "lisi": "李四",
    "wangwu": "王五"
  }
}
```

---

### POST /api/v1/contacts/sync

触发全量通讯录同步。异步执行。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "contact sync started",
    "running": true
  }
}
```

**错误响应：**

| code | msg |
|------|-----|
| 409 | contact sync already in progress |

---

### POST /api/v1/contacts/sync/incremental

触发增量通讯录同步。异步执行。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "contact incremental sync started",
    "running": true
  }
}
```

---

### POST /api/v1/contacts/sync/cancel

取消正在进行的通讯录同步任务。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "message": "contact sync cancellation requested"
  }
}
```

---

### GET /api/v1/contacts/sync/status

获取通讯录同步状态。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "running": false,
    "total": 350,
    "processed": 350
  }
}
```

---

## 8. 操作日志

### GET /api/v1/operation-logs

查询系统操作日志（API 调用记录）。

**Query 参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| action | string | - | 按操作类型过滤 |
| status_code | int | - | 按 HTTP 状态码过滤 |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 500,
    "data": [
      {
        "id": 1,
        "method": "POST",
        "path": "/api/v1/logs/query",
        "status_code": 200,
        "latency_ms": 45,
        "ip": "192.168.1.100",
        "created_at": "2024-01-01T12:00:00Z"
      }
    ]
  }
}
```

---

### GET /api/v1/operation-logs/actions

获取所有存在的操作类型列表（去重）。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": ["login", "query", "sync", "sync_cancel"]
}
```

---

## 9. 看板

### GET /api/v1/dashboard/overview

获取系统概览看板数据，包含 KPI 指标、最近同步记录和问题提醒。

**响应示例：**

```json
{
  "code": 0,
  "data": {
    "kpis": {
      "latest_sync_time": "2024-01-15T10:30:00Z",
      "latest_log_time": 1705312200,
      "synced_7d_count": 150000,
      "failed_feature_count": 0,
      "active_key_version": "v1",
      "active_key_days": 45,
      "key_count": 2,
      "contact_count": 350,
      "contact_last_sync": "2024-01-10T08:00:00Z",
      "inactive_rate": 12.5,
      "inactive_count": 44,
      "total_contacts": 350
    },
    "recent_syncs": [
      {
        "start_time": "2024-01-15T10:30:00Z",
        "sync_type": "log",
        "trigger": "scheduled",
        "succeeded": 12,
        "failed": 0,
        "duration_ms": 45000
      }
    ],
    "problems": [
      {
        "level": "warning",
        "message": "当前密钥版本已使用超过 90 天，建议轮换",
        "action": "keys"
      }
    ]
  }
}
```

**problems 级别说明：**

| level | 含义 |
|-------|------|
| error | 需要立即处理 |
| warning | 建议关注 |

**常见 problems 触发条件：**

- 最近一次同步有失败记录 → `level: error, action: sync`
- 密钥使用超过 90 天 → `level: warning, action: keys`
- 通讯录超过 7 天未同步 → `level: warning, action: contacts`
- 最近 24 小时无新日志入库 → `level: warning, action: sync`

---

### GET /api/v1/dashboard/inactive-users

查询未达标用户列表（活跃度不达标的人员）。

**Query 参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| range | string | quarter | 统计范围：`week`（近 7 天）、`month`（本月）、`quarter`（近 3 个月） |
| dept_id | int | - | 部门 ID 过滤 |
| min_inactive_days | int | total_days | 最少未活跃天数阈值 |

**响应示例：**

```json
{
  "code": 0,
  "data": {
    "total_contacts": 350,
    "inactive_count": 44,
    "inactive_users": [
      {
        "userid": "zhangsan",
        "name": "张三",
        "department": "[1,2]",
        "active_days": 30,
        "total_days": 90
      }
    ],
    "feature_names": {
      "90000031": "会话存档",
      "90000032": "日程"
    },
    "departments": [
      { "id": 1, "name": "总公司" }
    ],
    "dept_stats": [
      { "id": 1, "name": "总公司", "total": 350, "active": 306, "inactive": 44 }
    ],
    "range": "quarter",
    "total_days": 91,
    "min_inactive_days": 91
  }
}
```

---

## 10. 同步历史

### GET /api/v1/sync-history

查询同步任务执行历史。

**Query 参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数（上限 100） |
| sync_type | string | - | 同步类型过滤：`log`、`contact` |

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 50,
    "page": 1,
    "page_size": 20,
    "data": [
      {
        "id": 1,
        "sync_type": "log",
        "trigger": "scheduled",
        "start_time": "2024-01-15T10:30:00Z",
        "end_time": "2024-01-15T10:30:45Z",
        "succeeded": 12,
        "failed": 0,
        "duration_ms": 45000,
        "error_msg": ""
      }
    ]
  }
}
```

---

## 11. 数据类型配置

### GET /api/v1/sync-features

获取所有数据类型的同步启用状态。

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "feature_id": 90000031,
      "feature_name": "会话存档",
      "enabled": true,
      "last_sync": "2024-01-15T10:30:00Z"
    },
    {
      "feature_id": 90000066,
      "feature_name": "API 调用日志",
      "enabled": false,
      "last_sync": null
    }
  ]
}
```

---

### PUT /api/v1/sync-features

批量更新数据类型的启用/禁用状态。

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| features | object[] | 是 | 更新列表 |
| features[].feature_id | int | 是 | 数据类型 ID |
| features[].enabled | bool | 是 | 是否启用同步 |

**请求示例：**

```json
{
  "features": [
    { "feature_id": 90000031, "enabled": true },
    { "feature_id": 90000066, "enabled": false }
  ]
}
```

**响应示例：**

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    { "feature_id": 90000031, "feature_name": "会话存档", "enabled": true },
    { "feature_id": 90000066, "feature_name": "API 调用日志", "enabled": false }
  ]
}
```

---

## 12. 系统状态

### GET /api/v1/system/status

获取系统运行状态的完整概览。

**响应示例：**

```json
{
  "code": 0,
  "data": {
    "health": {
      "db_connected": true,
      "uptime_seconds": 86400
    },
    "sync_coverage": {
      "90000031": {
        "last_log_time": 1705312200,
        "last_sync_at": "2024-01-15T10:30:00Z",
        "total_synced": 50000,
        "data_age_hours": 2
      }
    },
    "table_sizes": [
      {
        "table": "log_90000031_202401",
        "rows": 50000,
        "data_bytes": 10485760,
        "index_bytes": 2097152
      }
    ],
    "key_status": {
      "active_version": "v1",
      "active_days": 45,
      "total_keys": 2
    },
    "contacts": {
      "total": 350,
      "last_sync": "2024-01-10T08:00:00Z",
      "sync_age_hours": 120
    }
  }
}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| health.db_connected | 数据库连接状态 |
| health.uptime_seconds | 服务运行时长（秒） |
| sync_coverage | 各数据类型的同步覆盖情况 |
| sync_coverage.*.data_age_hours | 最新数据距离现在的小时数（超过 48 小时表示数据较老） |
| table_sizes | 日志表大小排行（前 20，按行数降序） |
| key_status.active_key_days | 当前激活密钥的使用天数（超过 90 天建议轮换） |
| contacts.sync_age_hours | 通讯录距离上次同步的小时数 |
