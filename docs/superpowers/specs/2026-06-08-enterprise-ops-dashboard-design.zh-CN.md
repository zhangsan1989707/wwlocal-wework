# 企业级运营数据看板交付设计

## 目标

将现有运营数据看板提升到可交付的企业级基础版本，覆盖：

- 管理员或授权人员只能查看自己管理范围内的数据
- 登录人数、设备使用情况、使用人数、活跃人数、未使用人数、应用使用情况等指标
- 按日、周、月、季度统计
- CSV 导出，且导出数据与页面显示的数据范围一致

本设计目标是一个聚焦的企业交付基线，不是完整权限平台。

## 当前项目情况

项目已有真实数据基础：

- 政务微信日志同步和解密入库到按月日志表
- 通讯录同步到 `contacts`、`departments`、`contact_departments`
- `user_daily_stats` 保存用户每日 feature 活跃汇总
- Dashboard V2 预计算表和夜间统计任务
- Vue Dashboard V2 页面和部分 CSV 导出接口

阻塞正式交付的问题：

- 当前认证是单账号模式，JWT 只携带 `username`
- 没有 RBAC，也没有部门数据范围控制
- Dashboard V2 前后端响应结构不一致
- 周/月/季度人数指标存在把每日人数直接相加的风险
- 登录人数和按范围过滤的使用类指标不完整
- 导出能力没有稳定覆盖所有看板视图

## 本轮范围

本轮实现：

- 基础多账号认证
- 两个角色：`super_admin` 和 `dept_admin`
- 部门范围过滤，包含子部门
- Dashboard V2 概览、趋势、部门、设备、用户明细和导出接口都按权限范围过滤
- 超级管理员可用的最小用户管理 API 和页面，用于创建、禁用账号以及分配部门范围
- 修正 Dashboard V2 前端和后端接口契约
- 趋势支持日、周、月、季度
- 修正人数类指标聚合口径，避免重复计算
- 增加权限范围和统计聚合相关测试

本轮不做：

- 完整菜单权限和操作权限管理
- 超出最小账号/部门范围管理之外的完整用户后台
- 按具体应用名称拆分的深度应用分析，除非现有日志字段已经稳定提供应用维度
- 历史数据回填页面
- 外部 SSO 或企业身份系统集成

## 权限模型

新增两张表：

- `users`
  - `id`
  - `username`
  - `password_hash`
  - `role`
  - `enabled`
  - 时间戳
- `user_dept_scopes`
  - `user_id`
  - `dept_id`

角色定义：

- `super_admin`：可查看全部数据
- `dept_admin`：只能查看授权部门及其子部门的数据

启动兼容：

- 如果 `users` 表为空，则用现有 `AUTH_USERNAME` 和 `AUTH_PASSWORD` 初始化一个 `super_admin`
- 这样现有部署升级后不会丢失登录能力

JWT Claims：

- `user_id`
- `username`
- `role`
- `token_type`

Dashboard 相关接口不能信任前端传入的筛选条件做权限控制。每个受权限约束的接口都必须在服务端根据当前登录用户解析有效部门范围。

## 用户管理

增加一个最小的、仅 `super_admin` 可访问的用户管理能力，保证功能可运维，不需要直接改数据库。

后端接口：

- 用户列表
- 创建用户，包含角色和密码
- 更新用户角色、启用状态和部门范围
- 重置用户密码

前端：

- 增加一个紧凑的用户管理页面或区域，仅 `super_admin` 可见
- 支持创建 `dept_admin` 账号
- 支持为账号分配一个或多个部门作为范围根
- 支持禁用用户

这不是完整 RBAC 控制台，只管理本轮交付所需字段。

## 数据范围解析

对 `super_admin`：

- 不加部门过滤

对 `dept_admin`：

- 从 `user_dept_scopes` 读取直接授权部门
- 根据 `departments.parentid` 展开所有子部门
- 所有看板查询和导出都应用这个部门集合

如果 `dept_admin` 没有任何部门范围：

- 返回空数据，而不是返回全量数据

## 指标定义

人数类指标按所选周期内的用户去重计算：

- `login_users`：feature `90000031` 中的去重用户
- `usage_users`：活跃 feature 集合中的去重用户
- `active`：本轮与 `usage_users` 使用相同口径
- `inactive`：权限范围内有效通讯录人数减去范围内使用人数
- `activated`：feature `90000048` 中的去重用户
- `registered`：权限范围内有效通讯录人数

次数类指标按所选周期内事件数量求和：

- `msg_count`：消息相关 feature `90000035`、`90000036`、`90000037` 的日志行数
- `app_access_count`：feature `90000033` 的日志行数
- `group_created`：feature `90000038` 的日志行数
- `device_total`：feature `90000054` 中按设备统计的去重用户数

比例类指标用聚合后的分子和分母重新计算：

- 激活率 = 激活人数 / 注册人数
- 活跃率 = 活跃人数 / 注册人数

不能直接累加每日比例。

## 时间聚合

支持粒度：

- 日
- 周
- 月
- 季度

人数类指标：

- 按周期分组
- 在每个周期内对用户去重计数

次数类指标：

- 按周期分组
- 在每个周期内累计事件数量

这样可以避免周、月、季度统计中同一用户被重复计数。

## Dashboard V2 API 契约

保留现有路由族：

- `GET /api/v1/dashboard/v2/overview`
- `GET /api/v1/dashboard/v2/trend`
- `GET /api/v1/dashboard/v2/multi-trend`
- `GET /api/v1/dashboard/v2/departments`
- `GET /api/v1/dashboard/v2/devices`
- `GET /api/v1/dashboard/v2/users`
- `GET /api/v1/dashboard/v2/export/overview`
- `GET /api/v1/dashboard/v2/export/users`

按需新增受权限范围约束的导出接口：

- 趋势导出
- 部门导出
- 设备导出

概览响应结构：

```json
{
  "date": "2026-06-07",
  "registered": 100,
  "activated": 90,
  "not_activated": 10,
  "login_users": 80,
  "usage_users": 75,
  "active": 75,
  "inactive": 25,
  "rate_activation": 900,
  "rate_active": 750,
  "msg_count": 1000,
  "msg_sender": 70,
  "group_created": 8,
  "group_active": 4,
  "app_access_user": 30,
  "app_access_count": 200,
  "devices": {
    "total": 60,
    "types": [
      { "type": "device_android", "name": "Android", "count": 20, "percentage": 33.3 }
    ]
  },
  "scope": {
    "role": "dept_admin",
    "dept_ids": [2, 3]
  }
}
```

趋势响应结构：

```json
{
  "granularity": "month",
  "periods": ["2026-06"],
  "series": {
    "login_users": [80],
    "usage_users": [75]
  }
}
```

部门响应结构：

```json
[
  {
    "dept_id": 2,
    "dept_name": "Example",
    "total_contacts": 40,
    "active": 25,
    "inactive": 15,
    "active_rate": 62.5
  }
]
```

设备响应结构：

```json
{
  "date": "2026-06-07",
  "total": 60,
  "types": [
    { "type": "device_ios", "name": "iOS", "count": 10, "percentage": 16.7 }
  ]
}
```

## 前端设计

保留现有 Dashboard V2 页面。

需要调整：

- TypeScript 类型与后端响应结构对齐
- 使用后端真实支持的指标 key，例如 `login_users`、`usage_users`、`active`、`inactive`、`msg_count`、`app_access_count`
- 趋势粒度选择器增加“季度”
- 正确渲染 `devices.total` 和 `devices.types`
- 部门接口按数组消费
- 看板头部轻量展示当前角色/范围，让用户知道自己看到的是哪个范围
- CSV 导出调用后端受权限范围约束的导出接口

本轮不做新的落地页或装饰性 redesign。

## 导出设计

导出必须使用与页面一致的服务端权限范围。

必需导出：

- 概览 CSV
- 趋势 CSV
- 部门 CSV
- 设备 CSV
- 用户明细 CSV

用户明细导出支持列表类型：

- active
- inactive
- no_login

后端和前端统一使用 `no_login`，避免当前 `not_login` / `no_login` 混用。

## 错误处理

- 被禁用用户不能登录
- 用户名或密码错误返回未授权
- 没有部门范围的 `dept_admin` 返回空看板数据
- 日期或统计粒度非法时返回参数错误
- 导出接口遇到错误时返回错误，不能静默导出未授权范围的数据

## 测试

后端测试：

- `users` 为空时，可从现有认证配置初始化超级管理员
- 数据库用户可正常登录
- `super_admin` 解析为全量范围
- `dept_admin` 解析为授权部门及其子部门
- 没有范围的 `dept_admin` 解析为空范围
- 人数类指标按周期内用户去重
- 次数类指标按周期内事件求和
- Dashboard V2 service 对概览、趋势、用户、部门、设备、导出都应用权限范围

前端检查：

- `npm run build`
- Dashboard V2 不再读取不存在的后端字段
- 趋势图使用 `{periods, series}` 渲染
- 设备分布使用 `{total, types}` 渲染

通用验证：

- `go test ./...`
- `npm run build`

## 验收标准

- `super_admin` 可以看到全部部门和全量统计
- `dept_admin` 只能看到授权部门及其子部门
- 日、周、月、季度人数类指标都能正确去重
- 次数类指标在所选周期内正确求和
- Dashboard V2 面板不再读取不存在字段
- CSV 导出数据与页面显示范围一致
- 现有单管理员部署升级后仍可登录使用
