//go:build !windows

package sys

import "fmt"

func IsServiceAvailable() bool   { return false }
func IsServiceInstalled() bool   { return false }
func IsServiceRunning() bool     { return false }
func InstallZemService() error   { return fmt.Errorf("service mode only available on Windows") }
func UninstallZemService() error { return fmt.Errorf("service mode only available on Windows") }
func StartZemService() error     { return fmt.Errorf("service mode only available on Windows") }
func StopZemService() error      { return fmt.Errorf("service mode only available on Windows") }

// ServiceRunner 在非 Windows 平台不使用
type ServiceRunner struct {
	OnStart func() error
	OnStop  func()
}

func RunAsWindowsService(_ *ServiceRunner) error {
	return fmt.Errorf("service mode only available on Windows")
}
