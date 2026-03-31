# 部署 `autorun-go` 到 Vercel

## 1. 在 Vercel 中设置项目

- 将此仓库导入到 Vercel。
- 设置 **Root Directory** 为：
  - `autorun-go`

此项目配置为：
- `api/index.go` 作为 Go 函数入口。
- `vercel.json` 用于运行时和重写规则。

## 2. 环境变量

在 Vercel 项目设置 -> 环境变量 中设置这些：

- `RUN_PHONE`
- `RUN_PASSWORD`
- `ADMIN_TOKEN`（可选但推荐）
- `POSTGRES_URL`（或 `DATABASE_URL`）用于长期令牌存储
- `UPSTASH_REDIS_REST_URL` 用于 Redis 缓存
- `UPSTASH_REDIS_REST_TOKEN` 用于 Redis 认证

使用模式：
- 普通用户：在请求体中发送 `phone` + `password`。
- 管理员模式：发送 `adminToken`；后端将使用 `RUN_PHONE`/`RUN_PASSWORD`。
- 如果配置了 `POSTGRES_URL` + Redis 环境变量：
  - 登录将令牌写入 Postgres（持久化）和 Redis（缓存）
  - 后续请求先读 Redis，然后回退到 Postgres

## 3. API 端点

部署后，发送 POST 请求到：

- `https://<your-domain>/api`
- `https://<your-domain>/`（也通过重写规则支持）

请求体示例：

```json
{ "action": "login", "phone": "...", "password": "..." }
```

```json
{ "action": "club_data", "studentId": 123456, "queryDate": "2026-04-01" }
```

```json
{ "action": "club_join", "phone": "...", "password": "...", "activityId": 46994 }
```

```json
{ "action": "session_bootstrap", "studentId": 123456 }
```

## 4. 支持的操作

- `login` - 登录
- `run` - 运行
- `club` - 俱乐部
- `club_data` - 俱乐部数据
- `club_join` - 加入俱乐部
- `club_cancel` - 取消加入
- `session_bootstrap` - 会话启动
- `run_info` - 运行信息
- `club_join_num` - 俱乐部加入人数
- `club_top_three` - 俱乐部前三

## 5. 注意事项

- `map.json` 通过 `vercel.json` `includeFiles` 包含在 Go 函数运行时中。
- CORS 被启用为 `*` 以便于 PWA/API 集成。
- 响应中的 `tokenSrc` 标记令牌来源：`redis` / `database` / `login` / `relogin`。
