package sys

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// GetMacOSVersion 获取 macOS 版本
func GetMacOSVersion() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	cmd := exec.Command("sw_vers", "-productVersion")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(string(out))
}

// CheckMacOSPermissions 检查 macOS 网络扩展权限
func CheckMacOSPermissions() error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// 检查是否允许系统扩展
	cmd := exec.Command("spctl", "--status")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法检查系统安全策略: %w", err)
	}

	status := strings.TrimSpace(string(out))
	// assessments enabled 表示 Gatekeeper 正在评估，这是正常状态
	if strings.Contains(status, "assessments disabled") {
		fmt.Println("Warning: Gatekeeper assessments disabled")
	}

	return nil
}

// SetupMacOSDNS 配置 macOS DNS（用于 TUN 模式）
func SetupMacOSDNS(dnsServer string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// 获取主要网络接口
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "*") {
			continue
		}

		// 设置 DNS
		cmd = exec.Command("networksetup", "-setdnsservers", line, dnsServer)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set DNS for %s: %w", line, err)
		}
	}

	return nil
}

// ResetMacOSDNS 重置 macOS DNS
func ResetMacOSDNS() error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "*") {
			continue
		}

		// 重置为 DHCP
		cmd = exec.Command("networksetup", "-setdnsservers", line, "empty")
		cmd.Run() // 忽略错误
	}

	return nil
}

// GetMacOSUTUNName 获取可用的 utun 设备名
func GetMacOSUTUNName() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	// macOS 使用 utun 设备，通常从 utun0 开始
	// 但 sing-box 会自动处理，这里预留接口
	return "utun"
}
