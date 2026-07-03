# Zem

基于 Go + Wails + sing-box 的跨平台 VPN 客户端，支持 Clash 订阅格式。

## 特性

- 支持 Clash YAML、**sing-box JSON 原生**、**混合协议 URL 列表**（ss://, vmess://, vless://, trojan://, ssr://, hysteria2://, tuic://）订阅导入
- 支持 **Profile / 多订阅合并**，可选 union/select 合并模式
- 支持订阅高级选项：自定义 UA、Cookie、预处理、跳过 TLS 校验
- 本地订阅持久化存储
- 自动订阅更新（24小时间隔）
- 支持 VMess/VLESS/Trojan/Shadowsocks 等主流协议
- 支持 TUN 模式全局代理
- **跨平台支持**: Windows / macOS / Linux
- **Windows**: 管理员权限检测、wintun.dll 自动释放、防火墙规则自动配置、**可选 Windows 服务模式避免每次 UAC 弹窗**
- **TUN 模式**: 支持自定义地址、stack(gvisor/system/mixed)、MTU、strict_route 等参数
- **Linux**: 发行版检测、nftables/iptables 自动适配、TUN 模块自动安装
- **macOS**: 版本检测、DNS 自动配置、系统扩展权限检查

## 技术栈

- **后端**: Go + Wails v2 + sing-box
- **前端**: Vue 3 + Vite
- **协议**: VMess, VLESS, Trojan, Shadowsocks(SSR), Hysteria2, TUIC, WireGuard, AnyTLS, ShadowTLS, SOCKS5, HTTP, SSH

## 快速开始

### 前置要求

- Go 1.21+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 1. 安装依赖

```bash
cd frontend
npm install
cd ..
go mod tidy
```

### 2. Windows 准备 wintun.dll

```bash
# 方法1: 自动下载
bash download-wintun.sh

# 方法2: 手动下载
# 访问 https://www.wintun.net/ 下载 x64 版本
# 将 wintun.dll 放入 build/windows/
```

### 3. 开发模式

```bash
# Windows (管理员权限 CMD/PowerShell)
wails dev

# macOS
sudo wails dev

# Linux
sudo wails dev
```

### 4. 生产构建

```bash
# Windows
build.bat

# 或手动
wails build -ldflags "-s -w -buildid="

# macOS
wails build -platform darwin

# Linux
wails build -platform linux
```

## 项目结构

```
Zem/
├── main.go                      # 入口（集成平台特定初始化/清理）
├── wails.json                   # Wails 配置
├── go.mod                       # Go 依赖
├── build.sh / build.bat         # 构建脚本
├── download-wintun.sh           # wintun 自动下载脚本
├── build/
│   └── windows/
│       ├── Zem.exe.manifest   # 管理员权限清单
│       ├── wintun.dll           # TUN 驱动 (需自行下载)
│       └── README.md            # wintun 说明
├── internal/
│   ├── config/
│   │   ├── singbox_types.go     # sing-box 配置结构
│   │   └── clash2sing.go        # Clash 配置转换器
│   ├── engine/
│   │   └── singbox.go           # sing-box 引擎封装
│   ├── subscription/
│   │   └── manager.go           # 订阅管理器
│   └── sys/
│       ├── wintun.go            # 权限检测 + wintun释放
│       ├── wintun_embed.go      # wintun 嵌入支持（可选）
│       ├── tun.go               # 跨平台TUN/路由管理
│       ├── windows.go           # Windows 特定（服务/防火墙/代理）
│       ├── linux.go             # Linux 特定（发行版/防火墙/TUN）
│       └── macos.go             # macOS 特定（版本/DNS/utun）
└── frontend/
    └── src/
        └── App.vue              # 主界面（平台信息/权限状态）
```

## 使用说明

### Windows

1. **必须以管理员身份运行**（程序会检测并提示）
2. 首次运行会自动释放 `wintun.dll`
3. 程序会自动添加防火墙规则允许流量
4. 在"添加订阅"区域输入 Clash 订阅地址
5. 点击"添加订阅"，程序自动下载并转换配置
6. 在订阅列表中点击"连接"启动 VPN

### macOS

1. 使用 `sudo` 运行程序
2. 程序会自动配置 DNS 到 TUN 设备
3. 其余步骤与 Windows 相同
4. 断开时自动恢复 DNS 设置

### Linux

1. 使用 `sudo` 运行程序
2. 程序会自动检测发行版并配置防火墙（nftables/iptables）
3. 如果 TUN 模块未加载，会尝试自动安装
4. 其余步骤与 Windows 相同

## 权限说明

| 平台 | 要求 | 检测方式 | 自动处理 |
|------|------|----------|----------|
| Windows | 管理员权限 | 物理驱动器访问 | UAC 提权、防火墙规则 |
| macOS | root | UID 检测 | DNS 配置/恢复 |
| Linux | root/CAP_NET_ADMIN | UID 检测 | 防火墙、TUN 模块 |

## 平台特定功能

### Windows
- 管理员权限自动检测
- wintun.dll 自动释放（从嵌入资源或系统查找）
- 防火墙规则自动添加/移除
- 系统代理自动禁用/恢复
- 支持注册为 Windows 服务（可选）

### Linux
- 发行版自动检测（Debian/RedHat/Arch/Alpine）
- 防火墙自动适配（nftables/iptables）
- TUN 模块自动安装
- 路由自动配置/清理

### macOS
- 版本检测
- DNS 自动配置（networksetup）
- 系统扩展权限检查
- utun 设备支持

## 订阅地址格式示例

```
https://example.com/clash.yaml
https://106.55.228.246:36666/hxvip?token=xxx
```

## 数据存储位置

- **Windows**: `%APPDATA%\Zem\subscriptions\`
- **macOS/Linux**: `~/.config/Zem/subscriptions/`

## 故障排查

### "需要管理员权限" 提示
- Windows: 右键程序 → 以管理员身份运行
- macOS/Linux: 使用 `sudo` 运行

### TUN 设备创建失败
- Windows: 检查 wintun.dll 是否存在，运行 `download-wintun.sh`
- Linux: 运行 `sudo modprobe tun` 或 `sudo apt install tun-utils`
- macOS: 检查系统扩展权限

### 订阅更新失败
- 检查网络连接
- 检查订阅地址是否可用
- 查看数据目录下的日志

### Linux 防火墙问题
- 确保 nftables 或 iptables 已安装
- 程序会自动检测并使用可用的工具

### macOS DNS 不恢复
- 断开连接时会自动恢复
- 如果异常退出，手动运行 `networksetup -setdnsservers Wi-Fi empty`

## 构建优化

### 减小体积
```bash
# 去除符号表和调试信息
go build -ldflags="-s -w -buildid="

# 使用 UPX 压缩（可选）
upx --best Zem.exe
```

### 混淆（可选）
```bash
go install mvdan.cc/garble@latest
garble -literals -tiny build
```

## License

MIT
