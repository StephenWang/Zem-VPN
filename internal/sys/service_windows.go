//go:build windows

package sys

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	zemServiceName = "ZemCoreSvc"
	zemDisplayName = "Zem Core Service"
	zemServiceDesc = "Provides core networking support for Zem."
)

func IsServiceAvailable() bool {
	return true
}

func IsServiceInstalled() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()
	s, err := m.OpenService(zemServiceName)
	if err != nil {
		return false
	}
	s.Close()
	return true
}

func IsServiceRunning() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()
	s, err := m.OpenService(zemServiceName)
	if err != nil {
		return false
	}
	defer s.Close()
	status, err := s.Query()
	if err != nil {
		return false
	}
	return status.State == svc.Running
}

func InstallZemService() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()

	// 如果已存在则先卸载
	_ = uninstallService(m)

	s, err := m.CreateService(zemServiceName, exePath, mgr.Config{
		DisplayName: zemDisplayName,
		Description: zemServiceDesc,
		StartType:   mgr.StartAutomatic,
	}, "--service")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	s.Close()

	// 注册事件日志源（忽略错误）
	_ = eventlog.InstallAsEventCreate(zemServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	return nil
}

func UninstallZemService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	if err := uninstallService(m); err != nil {
		return err
	}
	_ = eventlog.Remove(zemServiceName)
	return nil
}

func uninstallService(m *mgr.Mgr) error {
	s, err := m.OpenService(zemServiceName)
	if err != nil {
		return nil // 未安装
	}
	defer s.Close()

	// 尝试停止服务
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		// 等待最多 10 秒
		for i := 0; i < 20; i++ {
			time.Sleep(500 * time.Millisecond)
			status, _ = s.Query()
			if status.State == svc.Stopped {
				break
			}
		}
	}
	return s.Delete()
}

func StartZemService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(zemServiceName)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()
	return s.Start()
}

func StopZemService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(zemServiceName)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()
	_, err = s.Control(svc.Stop)
	return err
}

// ServiceRunner 包装一个函数作为 Windows 服务运行
type ServiceRunner struct {
	OnStart func() error
	OnStop  func()
}

type zemService struct {
	runner *ServiceRunner
}

func (s *zemService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	changes <- svc.Status{State: svc.StartPending}

	runErr := make(chan error, 1)
	go func() {
		runErr <- s.runner.OnStart()
	}()

	// 等待启动完成或出错
	select {
	case err := <-runErr:
		if err != nil {
			changes <- svc.Status{State: svc.Stopped}
			return false, 1
		}
	case c := <-r:
		if c.Cmd == svc.Stop || c.Cmd == svc.Shutdown {
			s.runner.OnStop()
			changes <- svc.Status{State: svc.Stopped}
			return false, 0
		}
	}

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				s.runner.OnStop()
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}

// RunAsWindowsService 注册并运行 Windows 服务
func RunAsWindowsService(runner *ServiceRunner) error {
	return svc.Run(zemServiceName, &zemService{runner: runner})
}
