package sys

import "runtime"

// SetupPlatformConnection 在成功启动 sing-box 后调用，配置系统级代理/防火墙等。
// TUN 模式由 sing-box auto_route 接管系统路由；系统代理模式仅设置 HTTP 系统代理。
func SetupPlatformConnection(proxyAddr string, tunMode bool, mode string) error {
	switch runtime.GOOS {
	case "windows":
		if mode == "system" {
			return EnableWindowsProxy(proxyAddr)
		}
		return DisableWindowsProxy()
	case "linux":
		if tunMode {
			return SetupLinuxFirewall("tun0", true)
		}
		return nil
	}
	return nil
}

// CleanupPlatformConnection 在断开连接时调用，恢复系统设置
func CleanupPlatformConnection() error {
	switch runtime.GOOS {
	case "windows":
		_ = CleanupWindowsTUN()
		return DisableWindowsProxy()
	case "linux":
		return SetupLinuxFirewall("tun0", false)
	}
	return nil
}
