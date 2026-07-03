//go:build !windows

package sys

// EnableWindowsProxy 在非 Windows 平台上为空操作
func EnableWindowsProxy(proxyAddr string) error {
	return nil
}

// DisableWindowsProxy 在非 Windows 平台上为空操作
func DisableWindowsProxy() error {
	return nil
}
