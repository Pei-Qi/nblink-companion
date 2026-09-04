# 节点小宝固定端口伴侣

一个面向个人使用的 macOS/Windows 后台工具，为节点小宝“一键访问”服务提供稳定的本地端口。

当前版本：`0.3.0`，开发基线日期：`2026-09-03`。

## 技术栈

- Go `1.25.x`，项目固定 toolchain 为 `go1.25.14`。
- Wails `v3.0.0-beta.16`。
- Vue 3、TypeScript、Vite、Pinia 与 Lucide。
- 配置结构继续使用版本 2，新增 `themeMode` 字段兼容旧版回退。

## 功能

- 自动发现节点小宝本地 API 和账号下的服务卡片。
- 将 `127.0.0.1:固定端口` 透明转发到节点小宝生成的随机端口。
- 支持 HTTP、HTTPS、WebSocket、WSS、数据库、SSH、RDP、VNC 等 TCP 协议。
- 常用服务与当前运行状态分离：临时停止不会取消常用，取消常用不会中断当前连接。
- 新连接复用有效映射；15 秒健康检查检测节点小宝实例变化，后端拨号失败时自动重建映射。
- 支持搜索以及全部、常用、运行中、异常筛选，并提供复制、打开、编辑和启停操作。
- 支持原生轻盈白、原生轻盈黑和跟随系统三种主题模式。
- macOS 常驻菜单栏且不显示 Dock 图标；Windows 常驻系统托盘。启动后默认不弹出主窗口。
- 重复启动会唤醒已有实例并显示管理窗口。
- 支持登录启动、启动常用服务、自定义刷新周期及 RDP/VNC 客户端。
- 仅在已正常运行的规则掉线、发生错误或随后恢复时发送系统通知。
- 提供日志目录和不包含节点小宝凭据的诊断信息。
- 配置损坏时保留原文件备份并生成可用的默认配置。

## 使用

启动应用后，通过 macOS 菜单栏或 Windows 系统托盘图标打开管理窗口。关闭管理窗口只会隐藏界面，转发服务仍在后台运行；需要完全退出时使用托盘菜单中的“退出”。

默认配置路径：

- macOS：`~/Library/Application Support/nblink-companion/config.json`
- Windows：`%AppData%\nblink-companion\config.json`

## 开发

先安装前端依赖并生成生产资源：

```bash
cd frontend
npm ci
npm run build
cd ..
```

运行测试和桌面应用：

```bash
go test ./...
go run ./cmd/nblink-companion
```

前端可独立预览，浏览器模式使用完整模拟数据：

```bash
cd frontend
npm run dev
```

运行前端单元测试和桌面布局测试：

```bash
cd frontend
npm test
npx playwright install chromium
npm run test:e2e
```

运行转发吞吐、建连延迟和并发连接基准：

```bash
go test ./internal/proxy -run '^$' -bench . -benchmem
```

## macOS 打包

在 macOS 上执行：

```bash
./scripts/build-macos.sh
```

脚本执行 `npm ci` 和前端生产构建，使用唯一 SVG 源文件生成 `.icns`，再以 Wails `production` 标签分别编译 arm64 和 amd64 应用。输出：

```text
dist/macos/arm64/Nblink Companion.app
dist/macos/amd64/Nblink Companion.app
dist/Nblink-Companion-0.3.0-macos-arm64.zip
dist/Nblink-Companion-0.3.0-macos-amd64.zip
```

可以覆盖版本和构建号：

```bash
VERSION=0.3.0 BUILD_NUMBER=1 ./scripts/build-macos.sh
```

## Windows 打包

在 Windows 开发机上执行：

```powershell
.\scripts\build-windows.ps1
```

脚本执行前端生产构建，通过固定版本的 Wails v3 工具生成 Windows `.syso` 资源，再使用 Go 生成无控制台窗口的 `.exe`。默认输出：

```text
dist\windows\amd64\Nblink-Companion-0.3.0-windows-amd64.exe
```

## GitHub Actions 自动打包

仓库内置 `.github/workflows/windows-build.yml`，使用 GitHub 托管的 Windows Runner 完成前端测试、Go 测试和 Windows `amd64` 打包：

- 推送到 `main`、创建针对 `main` 的 Pull Request，或在 Actions 页面手动运行工作流时，会生成可下载的 Windows Artifact。
- 推送符合 `v主版本.次版本.修订版本` 格式的标签时，会在构建成功后自动创建 GitHub Release，并附带 `.exe` 和 SHA-256 校验文件。

发布当前版本：

```bash
git tag v0.3.0
git push origin v0.3.0
```

自动生成的 Windows 程序当前未进行代码签名。托盘、WebView2、登录启动、RDP/VNC 拉起和 ICO 显示仍需在 Windows 实机完成最终验收。

## 图标资源

- 唯一应用图标源文件：`assets/app-icon.svg`
- 唯一托盘图标源文件：`assets/tray-icon.svg`（高对比原色徽标，兼容明暗菜单栏）
- 应用构建资源：`assets/app-icon.png`、`assets/app-icon.icns`、`assets/app-icon.ico`
- 托盘构建资源：`assets/tray-icon.png`

可重新生成应用图标：

```bash
go run ./cmd/iconbuilder -svg ./assets/app-icon.svg -png ./assets/app-icon.png -icns ./assets/app-icon.icns -ico ./assets/app-icon.ico
go run ./cmd/iconbuilder -svg ./assets/tray-icon.svg -png ./assets/tray-icon.png -png-size 64
```

## 限制

- Wails v3 当前仍为 Beta，本项目固定到 `v3.0.0-beta.16`，升级前必须重新完成桌面验收。
- 当前只实现 TCP 固定端口，不转发 RDP 的可选 UDP 通道。
- 节点小宝或底层隧道中断时，已经建立的连接无法迁移；恢复后新连接会使用重建的映射。
- 远程唤醒依赖节点小宝未公开接口。请求格式有模拟测试覆盖，但节点小宝升级后仍可能需要更新适配器。
- 本项目不包含 Apple Developer ID 签名、公证、自动升级或遥测。
- Intel macOS 包可在 Apple Silicon 设备完成架构和静态检查，但最终启动验收仍应在 Intel Mac 上执行。

## 验收

自动化测试覆盖配置迁移与主题字段、快照 revision、并发刷新、规则编辑、唤醒目标解析、通知状态、常用规则和临时启停、API 探测、服务解析、双向透明转发、half-close、并发二进制传输，以及映射缓存和失败重建。

macOS 包可使用以下命令检查：

```bash
file 'dist/macos/arm64/Nblink Companion.app/Contents/MacOS/nblink-companion'
file 'dist/macos/amd64/Nblink Companion.app/Contents/MacOS/nblink-companion'
plutil -p 'dist/macos/arm64/Nblink Companion.app/Contents/Info.plist'
codesign --verify --deep --strict 'dist/macos/arm64/Nblink Companion.app'
```

Windows 平台代码保持条件编译覆盖；托盘、WebView2、登录启动、RDP/VNC 拉起和 ICO 显示仍需在 Windows 实机完成最终验收。
