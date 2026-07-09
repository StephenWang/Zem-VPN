package platform

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"zem/internal/settings"
	"zem/internal/sys"
)

// Manager 封装平台特定的初始化、清理与连接/断开时的平台级配置。
type Manager struct {
	settings *settings.Manager
	dataDir  string
}

func NewManager(settings *settings.Manager, dataDir string) *Manager {
	return &Manager{
		settings: settings,
		dataDir:  dataDir,
	}
}

// Init 在应用启动时执行平台特定初始化（权限、驱动、防火墙等）。
func (m *Manager) Init() {
	switch runtime.GOOS {
	case "windows":
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}
		if _, err := sys.ExtractWintun(); err != nil {
			fmt.Println("Wintun:", err)
		}
		exePath, _ := os.Executable()
		sys.AddWindowsFirewallRule("Zem", exePath)

	case "linux":
		if err := sys.CheckTUNSupport(); err != nil {
			fmt.Println("Warning:", err)
			fmt.Println("尝试安装 TUN 模块...")
			if err := sys.InstallTUNModule(); err != nil {
				fmt.Println("安装失败:", err)
			}
		}
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}

	case "darwin":
		if err := sys.CheckMacOSPermissions(); err != nil {
			fmt.Println("Warning:", err)
		}
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}
	}
}

// Cleanup 在应用退出时执行平台特定清理。
func (m *Manager) Cleanup() {
	switch runtime.GOOS {
	case "windows":
		sys.RemoveWindowsFirewallRule("Zem")
	case "darwin":
		sys.ResetMacOSDNS()
	}
	sys.CleanupRoutes("tun0")
}

// Apply 在连接/断开时应用平台级设置。
func (m *Manager) Apply(connected bool) error {
	if connected {
		proxyAddr := fmt.Sprintf("127.0.0.1:%d", m.settings.GetProxyPort())
		mode := m.settings.GetProxyMode()
		tunMode := mode != "system" && m.settings.GetTunSettings().AutoRoute
		if err := sys.SetupPlatformConnection(proxyAddr, tunMode, mode); err != nil {
			fmt.Println("platform connection setup:", err)
			return err
		}
		if runtime.GOOS == "darwin" && mode != "system" {
			_ = sys.SetupMacOSDNS(m.tunDNSAddress())
		}
		return nil
	}

	if err := sys.CleanupPlatformConnection(); err != nil {
		fmt.Println("platform connection cleanup:", err)
		return err
	}
	if runtime.GOOS == "darwin" {
		_ = sys.ResetMacOSDNS()
	}
	return nil
}

// tunDNSAddress 浠巌UN CIDR 鎺ㄥ DNS 鏈嶅姟鍣ㄥ湴鍧€
func (m *Manager) tunDNSAddress() string {
	tun := m.settings.GetTunSettings()
	if len(tun.Address) == 0 {
		return "172.19.0.2"
	}

	ip, ipNet, err := net.ParseCIDR(tun.Address[0])
	if err != nil {
		return "172.19.0.2"
	}

	network := ipNet.IP.Mask(ipNet.Mask)
	var candidates []net.IP
	for offset := 1; offset <= 2; offset++ {
		addr := make([]byte, len(network))
		copy(addr, network)
		for i := 0; i < offset; i++ {
			for j := len(addr) - 1; j >= 0; j-- {
				addr[j]++
				if addr[j] != 0 {
					break
				}
			}
		}
		if ipNet.Contains(addr) {
			candidates = append(candidates, addr)
		}
	}

	if len(candidates) < 2 {
		return "172.19.0.2"
	}

	for _, addr := range candidates {
		if !addr.Equal(ip) {
			return addr.String()
		}
	}
	return candidates[1].String()
}
