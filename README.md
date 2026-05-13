# AutoRun 本地版

这个项目现在按本地全栈应用组织：Go 后端负责 API，React 前端放在 `autorun-go-pwa/`，生产包里由 Go 直接托管前端静态文件。

平台化适配只处理启动、打包和本地路由，不改变登录、签名、轨迹生成、跑步提交、俱乐部签到等原有算法和业务逻辑。

详细模块关系、请求流程和定时签到/签退机制见 `ARCHITECTURE.md`。

## 给小白用户

开发者打包后，把 `release/AutoRun-*.zip` 发给用户。

当前仓库也保留了已打包好的 zip：

- `release/AutoRun-darwin-arm64.zip`
- `release/AutoRun-darwin-amd64.zip`
- `release/AutoRun-windows-amd64.zip`
- `release/AutoRun-linux-amd64.zip`

用户解压后：

- Windows：双击 `start-autorun.bat`
- macOS：双击 `start-autorun.command`
- Linux：运行 `./start-autorun.sh`

程序会自动启动本地服务并打开：

```text
http://localhost:8080
```

用户只需要在页面里输入手机号和密码使用。

如果用户直接双击 `autorun` 或 `autorun.exe`，程序也会启动服务并自动打开浏览器。命令行窗口保持打开是正常现象，关闭窗口或按 `Ctrl+C` 就会停止本地服务。

本地版会监听页面生命周期：用户关闭最后一个 AutoRun 浏览器页面后，后端会自动优雅退出；刷新页面时会有短暂缓冲，不会误关服务。

## 给开发者打包

在项目根目录执行：

```bash
./scripts/package-local.sh
```

脚本会完成：

1. 安装或复用前端依赖。
2. 以 `VITE_API_BASE=/api` 构建 React 前端。
3. 编译 Go 后端。
4. 生成本地发行包。

输出目录：

```text
release/
├── AutoRun-darwin-arm64/
├── AutoRun-darwin-amd64/
├── AutoRun-windows-amd64/
├── AutoRun-linux-amd64/
└── *.zip
```

发行包结构：

```text
AutoRun-<platform>/
├── autorun 或 autorun.exe
├── web/
│   ├── index.html
│   └── assets/
├── start-autorun.*
└── README.txt
```

Go 程序启动时会自动识别同目录下的 `web/`，所以发行包不需要额外配置 `FRONTEND_DIST`。

## 本地开发

同时启动 Go 后端和 Vite 前端：

```bash
./scripts/dev-local.sh
```

默认端口：

```text
Go API:   http://localhost:8080/api
Vite Web: http://localhost:5173
```

前端开发模式下，Vite 会把 `/api` 代理到 `http://localhost:8080`。

## 只启动后端

```bash
go run .
```

接口入口：

```text
POST http://localhost:8080/api
POST http://localhost:8080/api/<action>
```

如果要让 Go 直接托管当前前端构建产物：

```bash
cd autorun-go-pwa
VITE_API_BASE=/api pnpm run build

cd ..
FRONTEND_DIST=autorun-go-pwa/dist go run .
```

打开：

```text
http://localhost:8080
```

## 可选环境变量

可以参考 `.env.example`。如果使用 `.env.local` 存后端环境变量，Go 不会自动读取它，需要这样启动：

```bash
set -a
source .env.local
set +a
go run .
```

常用变量：

- `PORT`：后端端口，默认 `8080`。
- `FRONTEND_DIST`：React 构建产物目录，配置后 Go 会托管前端页面。
- `AUTO_OPEN_BROWSER`：设置为 `0`、`false` 或 `no` 时，不自动打开浏览器。
- `AUTORUN_DATA_DIR`：本地 session 数据目录，默认使用系统用户配置目录。
- `SESSION_STORE_PATH`：本地 session JSON 文件路径，优先级高于 `AUTORUN_DATA_DIR`。
- `RUN_PHONE` / `RUN_PASSWORD` / `ADMIN_TOKEN`：管理员内置账号模式，可不配。

登录态缓存使用本地 JSON 文件，不需要 Postgres、Redis 或其他外部服务。
