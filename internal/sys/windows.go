package sys

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// RegisterWindowsService 注册为 Windows 服务
func RegisterWindowsService(serviceName, displayName, exePath string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("sc.exe", "create", serviceName,
		"binPath=", exePath,
		"DisplayName=", displayName,
		"start=", "auto",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("register service: %w, output: %s", err, string(out))
	}

	return nil
}

// UnregisterWindowsService 卸载 Windows 服务
func UnregisterWindowsService(serviceName string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("sc.exe", "delete", serviceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unregister service: %w, output: %s", err, string(out))
	}

	return nil
}

// AddWindowsFirewallRule 添加 Windows 防火墙规则
func AddWindowsFirewallRule(ruleName, exePath string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=", ruleName,
		"dir=", "in",
		"action=", "allow",
		"program=", exePath,
		"enable=", "yes",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add firewall rule: %w, output: %s", err, string(out))
	}

	return nil
}

// RemoveWindowsFirewallRule 移除 Windows 防火墙规则
func RemoveWindowsFirewallRule(ruleName string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=", ruleName,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		// 规则可能不存在，忽略错误
		if strings.Contains(string(out), "No rules match") {
			return nil
		}
		return fmt.Errorf("remove firewall rule: %w, output: %s", err, string(out))
	}

	return nil
}
