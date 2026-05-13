#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/autorun-go-pwa"
RELEASE_DIR="$ROOT_DIR/release"
APP_NAME="AutoRun"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1"
    exit 1
  fi
}

frontend_runner() {
  if command -v pnpm >/dev/null 2>&1; then
    echo "pnpm"
    return
  fi
  if command -v npm >/dev/null 2>&1; then
    echo "npm"
    return
  fi
  echo "Missing pnpm or npm"
  exit 1
}

install_frontend_deps() {
	local runner="$1"
	cd "$FRONTEND_DIR"
	if [ -d node_modules ]; then
		echo "Reusing existing frontend node_modules."
		return
	fi
	if [ "$runner" = "pnpm" ]; then
		if [ -f pnpm-lock.yaml ]; then
			CI=true pnpm install --frozen-lockfile
		else
			CI=true pnpm install
		fi
	else
		CI=true npm install
	fi
}

build_frontend() {
  local runner="$1"
  cd "$FRONTEND_DIR"
  VITE_API_BASE=/api "$runner" run build
}

write_windows_launcher() {
  local out_dir="$1"
  cat > "$out_dir/start-autorun.bat" <<'BAT'
@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
if "%PORT%"=="" set PORT=8080
start "AutoRun Server" /min "%~dp0autorun.exe"
echo AutoRun 正在启动：http://localhost:%PORT%
echo 请从本文件或 autorun.exe 启动，不要只从 Windows PWA 图标启动。
echo 关闭最后一个 AutoRun 页面后，后端会自动退出。
BAT
}

write_unix_launcher() {
  local out_dir="$1"
  local binary_name="$2"
  cat > "$out_dir/start-autorun.sh" <<SH
#!/usr/bin/env bash
set -euo pipefail
cd "\$(dirname "\$0")"
PORT="\${PORT:-8080}"
export PORT
./${binary_name} > autorun.log 2>&1 &
SERVER_PID=\$!
cleanup() {
  kill "\$SERVER_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
echo "AutoRun is running at http://localhost:\${PORT}"
echo "Press Ctrl+C to stop."
wait "\$SERVER_PID"
SH
  chmod +x "$out_dir/start-autorun.sh"
  if [ "$(basename "$out_dir")" = "${APP_NAME}-darwin-arm64" ] || [ "$(basename "$out_dir")" = "${APP_NAME}-darwin-amd64" ]; then
    cp "$out_dir/start-autorun.sh" "$out_dir/start-autorun.command"
    chmod +x "$out_dir/start-autorun.command"
  fi
}

write_package_readme() {
  local out_dir="$1"
  cat > "$out_dir/README.txt" <<'TXT'
AutoRun 本地版使用说明

一、Windows 用户

1. 解压 zip 后，双击 start-autorun.bat。
2. 程序会启动本地后端，并自动打开 http://localhost:8080。
3. 如果你安装过 Windows PWA 图标，不要只从 PWA 图标启动。
   PWA 只是浏览器应用，不能自己拉起本地后端。
4. 如果 PWA 页面提示“本地服务未启动”，请回到本文件所在目录，
   双击 start-autorun.bat 或 autorun.exe。

二、macOS 用户

1. 双击 start-autorun.command。
2. 如果系统提示无法打开，可以在终端运行 ./start-autorun.sh。

三、Linux 用户

在终端运行：

  ./start-autorun.sh

四、关闭方式

关闭最后一个 AutoRun 浏览器页面后，后端会自动退出。
如果你是从终端启动，也可以按 Ctrl+C 停止。

五、账号说明

手机号和密码在网页里输入。本地版不需要 Postgres、Redis 或云端数据库。
登录态和定时配置只保存在当前电脑的本地用户配置目录中。
TXT
}

package_target() {
  local goos="$1"
  local goarch="$2"
  local binary_name="$3"
  local package_name="${APP_NAME}-${goos}-${goarch}"
  local out_dir="$RELEASE_DIR/$package_name"

  rm -rf "$out_dir"
  mkdir -p "$out_dir/web"
  cp -R "$FRONTEND_DIR/dist/." "$out_dir/web/"

  cd "$ROOT_DIR"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="-s -w" -o "$out_dir/$binary_name" .

  if [ "$goos" = "windows" ]; then
    write_windows_launcher "$out_dir"
  else
    chmod +x "$out_dir/$binary_name"
    write_unix_launcher "$out_dir" "$binary_name"
  fi
  write_package_readme "$out_dir"

  cd "$RELEASE_DIR"
  if command -v zip >/dev/null 2>&1; then
    rm -f "$package_name.zip"
    zip -qr "$package_name.zip" "$package_name"
  fi
}

main() {
  require_command go
  local runner
  runner="$(frontend_runner)"

  install_frontend_deps "$runner"
  build_frontend "$runner"

  rm -rf "$RELEASE_DIR"
  mkdir -p "$RELEASE_DIR"

  package_target darwin arm64 autorun
  package_target darwin amd64 autorun
  package_target windows amd64 autorun.exe
  package_target linux amd64 autorun

  echo "Local packages are ready in: $RELEASE_DIR"
}

main "$@"
