# 部署 `autorun-go` 到 Vercel

> 当前项目已改成本地版优先。登录态缓存使用本地 JSON 文件，适合桌面/本机运行；Vercel 这类无服务器环境的本地文件不保证持久化，因此不再作为推荐部署方式。

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
- `AUTORUN_DATA_DIR`（可选）用于指定本地 session 数据目录
- `SESSION_STORE_PATH`（可选）用于指定本地 session JSON 文件路径

使用模式：
- 普通用户：在请求体中发送 `phone` + `password`。
- 管理员模式：发送 `adminToken`；后端将使用 `RUN_PHONE`/`RUN_PASSWORD`。
- 登录态默认写入本地 JSON 文件；不再需要 Postgres 或 Redis。

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
- 响应中的 `tokenSrc` 标记令牌来源：`local` / `login` / `relogin`。
