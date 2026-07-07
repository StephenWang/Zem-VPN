package platform

import (
	"fmt"
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
			_ = sys.SetupMacOSDNS("172.19.0.2")
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
