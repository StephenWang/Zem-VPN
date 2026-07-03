package sys

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// SetupTUN 配置 TUN 设备
func SetupTUN(ifaceName string, ip string, subnet int) error {
	switch runtime.GOOS {
	case "windows":
		return setupTUNWindows(ifaceName, ip, subnet)
	case "darwin":
		return setupTUNMacOS(ifaceName, ip, subnet)
	case "linux":
		return setupTUNLinux(ifaceName, ip, subnet)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func setupTUNWindows(ifaceName, ip string, subnet int) error {
	// Windows 下 sing-box TUN 自动配置，这里预留扩展
	return nil
}

func setupTUNMacOS(ifaceName, ip string, subnet int) error {
	cmd := exec.Command("ifconfig", ifaceName, "inet", ip, ip, "up")
	return cmd.Run()
}

func setupTUNLinux(ifaceName, ip string, subnet int) error {
	cmd := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", ip, subnet), "dev", ifaceName)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("ip", "link", "set", ifaceName, "up")
	return cmd.Run()
}

// AddRoute 添加路由
func AddRoute(ifaceName string, dest string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("route", "add", dest, "mask", "255.255.255.255", "0.0.0.0", "if", ifaceName)
		return cmd.Run()
	case "darwin", "linux":
		cmd := exec.Command("ip", "route", "add", dest, "dev", ifaceName)
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// CleanupRoutes 清理路由 (程序退出时调用)
func CleanupRoutes(ifaceName string) error {
	if runtime.GOOS == "windows" {
		// Windows 下 TUN 关闭时自动清理
		return nil
	}

	// Linux/macOS 可能需要手动清理
	cmd := exec.Command("ip", "route", "flush", "dev", ifaceName)
	cmd.Run() // 忽略错误

	return nil
}

// CleanupWindowsTUN 清理 Windows 下残留的 TUN 网络适配器
func CleanupWindowsTUN() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	// 使用 PowerShell 禁用并删除 wintun 创建的 TUN 适配器
	ps := `
$adapters = Get-NetAdapter | Where-Object { $_.InterfaceDescription -like "*wintun*" -or $_.Name -like "*sing*" -or $_.Name -like "*Zem*" }
foreach ($a in $adapters) {
    try { Disable-NetAdapter -Name $a.Name -Confirm:$false } catch {}
    try { Remove-NetAdapter -Name $a.Name -Confirm:$false } catch {}
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No MSFT_NETAdapter") {
		return fmt.Errorf("cleanup windows tun: %w, output: %s", err, string(out))
	}
	return nil
}

// GetDefaultGateway 获取默认网关
func GetDefaultGateway() (string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "route print 0.0.0.0 | findstr 0.0.0.0")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == "0.0.0.0" {
				return fields[2], nil
			}
		}
		return "", fmt.Errorf("gateway not found")
	case "darwin":
		cmd := exec.Command("netstat", "-rn", "-f", "inet")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "default") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					return fields[1], nil
				}
			}
		}
		return "", fmt.Errorf("gateway not found")
	case "linux":
		cmd := exec.Command("ip", "route", "show", "default")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "via" && i+1 < len(fields) {
				return fields[i+1], nil
			}
		}
		return "", fmt.Errorf("gateway not found")
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
