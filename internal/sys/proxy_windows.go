//go:build windows

package sys

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
)

// EnableWindowsProxy 启用 Windows 系统代理（HTTP 代理模式）
func EnableWindowsProxy(proxyAddr string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings registry: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", proxyAddr); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}

	// 清除自动配置脚本，避免PAC覆盖手动代理
	_ = k.DeleteValue("AutoConfigURL")

	refreshProxySettings()
	return nil
}

// DisableWindowsProxy 禁用 Windows 系统代理
func DisableWindowsProxy() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings registry: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}

	refreshProxySettings()
	return nil
}

// refreshProxySettings 通知 Windows 应用代理设置已变更
func refreshProxySettings() {
	wininet := windows.NewLazySystemDLL("wininet.dll")
	proc := wininet.NewProc("InternetSetOptionW")
	// INTERNET_OPTION_SETTINGS_CHANGED = 39
	proc.Call(0, 39, 0, 0)
	// INTERNET_OPTION_REFRESH = 37
	proc.Call(0, 37, 0, 0)
	// INTERNET_OPTION_PROXY_SETTINGS_CHANGED = 95
	proc.Call(0, 95, 0, 0)
}
