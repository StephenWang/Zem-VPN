# Agent 指南

本文件为 Kimi Code CLI 等编码代理提供项目背景、构建步骤、代码约定和常见注意事项。

## 1. 项目概述

这是一个基于 **Go + Wails v2** 开发的桌面 VPN 客户端，内部使用 **sing-box** 作为核心代理/TUN 引擎。项目定位是将 Clash 格式的订阅转换为 sing-box JSON 配置，并通过 TUN 模式实现系统级代理。

- **Go 模块名**: `zem`
- **Wails 应用名**: `Zem`
- **前端框架**: Vue 3 + Vite 4 + vue-router
- **支持平台**: Windows、Linux、macOS

> 项目名称已统一为 `Zem`。Go 模块路径使用小写 `zem`（Go 模块路径约定），其余面向用户的名称、数据目录、构建产物均使用 `Zem`。

## 2. 项目结构

```
Zem/
├── main.go                      # 程序入口 / Wails 应用 / 暴露给前端的方法
├── wails.json                   # Wails v2 配置
├── go.mod                       # Go 模块与依赖声明
├── build.bat                    # Windows 构建脚本
├── build.sh                     # 跨平台构建脚本
├── download-wintun.sh           # 下载 wintun.dll 脚本
├── README.md
├── build/
│   └── windows/
│       ├── wails.exe.manifest   # 管理员权限清单（Wails 构建/开发均使用此文件）
│       └── README.md
├── internal/
│   ├── config/                  # Clash YAML → sing-box JSON 配置转换
│   ├── engine/                  # sing-box 引擎封装（Start/Stop/Status）
│   ├── settings/                # 应用设置持久化（代理端口等）
│   ├── subscription/            # 订阅下载、转换、持久化与自动更新
│   └── sys/                     # 平台相关系统工具（权限、TUN、防火墙、DNS）
└── frontend/
    ├── index.html
    ├── package.json
    ├── vite.config.js
    └── src/
        ├── main.js
        ├── App.vue              # 应用主框架（含 Sidebar + router-view）
        ├── router/              # vue-router 路由配置
        ├── components/          # 可复用组件（Sidebar 等）
        ├── elements/            # 基础元素组件（Icon.vue 等）
        ├── icons/               # SVG 图标组件
        └── views/               # 路由页面组件
            ├── Servers.vue
            ├── Subscriptions.vue
            └── Configuration.vue
```

## 3. 核心模块职责

### 3.1 `main.go`
- 定义 `App` 结构体，聚合 `engine.SingBoxEngine`、`subscription.Manager` 和 `dataDir`。
- 实现 Wails 生命周期钩子 `Startup`、`Shutdown`。
- 暴露给前端的绑定方法：订阅管理（Add/Connect/Update/Delete/List/GetConfig）、服务器管理（GetServers/SelectServer/SpeedTest/GetCurrentSubscriptionID）、连接控制（Disconnect/GetStatus）、代理端口设置（GetProxyPort/SetProxyPort）、系统信息（IsAdmin/GetPlatformInfo）。
- 使用 `//go:embed all:frontend/dist` 嵌入前端构建产物。

### 3.2 `internal/config/`
- `singbox_types.go`: sing-box JSON 配置结构体（已适配 sing-box 1.14 格式）。
- `clash2sing.go`: 将 Clash YAML 订阅转换为 sing-box JSON，支持 vmess/vless/trojan/shadowsocks/http/socks5 等协议。
- 当前 sing-box 版本为 **v1.14.0-alpha.31**；GeoIP/Geosite 规则因需要 rule-set 暂被跳过，仅 domain/IP/PORT/PROCESS 类规则生效。

### 3.3 `internal/engine/`
- `singbox.go`: 封装 sing-box `box.Box` 实例。
- 启动时使用 `include.Context()` 注册 inbound/outbound/endpoint/DNS/service registry，否则 sing-box 1.12+ 会报 `missing endpoint registry in context`。
- 配置解析使用 `github.com/sagernet/sing/common/json.UnmarshalExtendedContext`，以兼容 sing-box 1.12+ 的 context-aware JSON 反序列化。
- 提供 `Start`、`Stop`、`Status`、`GetLastConfig`、`GetCurrentSubID`、`SetCurrentSubID`。

### 3.4 `internal/subscription/`
- 订阅元数据内存存储 + 本地 JSON 文件持久化。
- 订阅 ID 为 URL MD5 前 8 位。
- 下载订阅时使用 UA `ClashforWindows/0.20.39`，并关闭 TLS 证书校验。
- 自动更新间隔：24 小时。
- `Save` 方法也会将转换后的 sing-box JSON 写入 `<id>_sing.json`。

### 3.5 `internal/settings/`
- 应用设置持久化，当前仅保存 `proxy_port`（默认 `7890`）。
- 连接订阅时，`main.go` 的 `prepareConfig` 会注入/更新 `mixed` 入站端口。

### 3.6 `internal/sys/`
- `wintun.go`: Windows TUN 驱动（wintun.dll）相关。
- `tun.go`: 跨平台 TUN/路由辅助。
- `windows.go`: Windows 服务、防火墙、系统代理。
- `linux.go`: Linux 发行版检测、iptables/nftables、TUN 模块安装。
- `macos.go`: macOS 版本、权限、DNS 配置。

## 4. 构建说明

### 4.1 首次构建前必须执行

```bash
# 1. 安装前端依赖并构建
 cd frontend
 npm install
 npm run build
 cd ..

# 2. 生成 Go 依赖锁
 go mod tidy

# 3. 生成 Wails 前端绑定（会生成 frontend/src/wailsjs/）
 wails generate module
```

### 4.2 Windows 构建

```bash
# 方式 1：使用 build.bat（需安装 Wails CLI 和 Go）
build.bat

# 方式 2：直接使用 Wails
wails build -ldflags "-s -w -buildid=" -o "dist\Zem.exe"
```

> Windows 开发/运行需要管理员权限，且需要 `wintun.dll`。可运行 `download-wintun.sh` 下载，或手动从 https://www.wintun.net/ 获取并放置到 `build/windows/wintun.dll`。

### 4.3 跨平台构建

```bash
bash build.sh
```

该脚本会构建 Windows（x86_64-w64-mingw32-gcc 交叉编译）、Linux、macOS（amd64/arm64）二进制。

## 5. 开发约定

### 5.1 Go 代码
- 包名：小写，无下划线（`config`、`engine` 等）。
- 导出类型/方法使用 PascalCase。
- 文件名使用小写+下划线（`clash2sing.go`、`wintun_embed.go`）。
- 错误处理优先使用 `fmt.Errorf("...: %w", err)`。
- 中文注释较多，修改时保持一致风格。

### 5.2 前端代码
- 使用 Vue 3 Composition API（`<script setup>`）。
- 原生 CSS，无 UI 组件库。
- 通过 `frontend/src/wailsjs/go/main/App.js` 调用后端绑定方法。
- **不要手动修改 `frontend/src/wailsjs/` 下的自动生成文件**，应通过 `wails generate module` 或 `wails dev` 重新生成。

### 5.3 平台特定代码
- Windows 相关代码放在 `internal/sys/windows.go` 和 `wintun.go`。
- Linux 相关代码放在 `internal/sys/linux.go`。
- macOS 相关代码放在 `internal/sys/macos.go`。
- 新增系统操作时，请按 `runtime.GOOS` 分文件实现，避免在一个文件内写大量 `if runtime.GOOS == ...`。

## 6. 已知问题与注意事项

### 6.1 构建前置条件
- 首次构建前需先执行 `go mod tidy` 生成 `go.sum`。
- 需先构建前端生成 `frontend/dist`（`cd frontend && npm install && npm run build`）。
- 若未生成 `frontend/src/wailsjs/`，需运行 `wails generate module`。
- **Wails 应用必须使用 `wails build` 构建**，直接使用 `go build` 会缺少 production 构建标签，运行时报错。
- 缺少 `frontend/dist` 时直接 `go build` 会报 `pattern all:frontend/dist: no matching files found`。

### 6.2 逻辑问题
- `download-wintun.sh` 已修复：使用 `mktemp` 并在脚本所在目录的项目根目录下释放 `wintun.dll`。
- `internal/sys/wintun_embed.go` 已改为 `//go:build embedwintun`。
- 国家/地区信息当前通过节点名称关键词猜测，未使用 IP GeoIP 数据库，准确性有限。

### 6.3 新增功能
- 订阅格式：已支持 Clash YAML、sing-box JSON 原生、ss://、vmess://、vless://、trojan://、ssr://、hysteria2://、tuic:// 及 base64 编码的混合协议列表。
- Profile：支持将多个订阅合并为一个 Profile（union/select 模式）。
- 订阅高级选项：自定义 UA、Cookie、预处理（base64）、跳过 TLS 校验。

### 6.4 安全与维护提示
- 订阅下载默认启用 TLS 校验；如果订阅源证书异常，可设置环境变量 `ZEM_SKIP_TLS_VERIFY=1` 或在订阅选项中勾选跳过校验。
- 项目已包含 `main_test.go`、`internal/config/*_test.go`、`internal/subscription/manager_test.go`、`internal/settings/settings_test.go`、`internal/profile/manager_test.go` 等测试文件，新增核心逻辑时建议继续补充测试。
- 已添加 `.gitignore`，避免提交 `frontend/dist`、`dist/`、`build/bin/` 等构建产物。
- 无 CI/CD 配置。
- 日志使用 `fmt.Println` 输出到 stdout，未持久化到文件。

### 6.5 硬编码值
修改以下值时需评估影响：
- TUN 地址：`172.19.0.1/30`
- macOS DNS 服务器：`172.19.0.2`
- 数据目录：Windows `%APPDATA%\Zem`，macOS/Linux `~/.config/Zem`
- 订阅更新间隔：`24 * time.Hour`
- 订阅 UA：`ClashforWindows/0.20.39`
- Windows 防火墙规则名/服务名：`Zem`

## 7. 给代理的操作建议

- 不要直接执行 `go build` / `go build ./...` 来产出可执行文件，必须使用 `wails build`；`go build` 仅适合快速检查 Go 代码编译，生成的二进制无法正常运行。
- 修改 `main.go` 中暴露给前端的函数签名后，必须运行 `wails generate module` 或 `wails dev` 更新 `frontend/src/wailsjs/`。
- 修改 `internal/sys/` 时请优先在对应平台文件操作，并注意字符串转义。
- 新增订阅协议支持时，修改 `internal/config/clash2sing.go`，并同步更新 `internal/config/singbox_types.go` 中相关结构体。
- 项目名称已统一为 `Zem`。Go 模块路径保持小写 `zem`。
