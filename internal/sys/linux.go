package sys

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetLinuxDistro 检测 Linux 发行版
func GetLinuxDistro() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// 读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	content := string(data)
	if strings.Contains(content, "ID=ubuntu") || strings.Contains(content, "ID=debian") {
		return "debian"
	}
	if strings.Contains(content, "ID=fedora") || strings.Contains(content, "ID=rhel") || strings.Contains(content, "ID=centos") {
		return "redhat"
	}
	if strings.Contains(content, "ID=arch") || strings.Contains(content, "ID=manjaro") {
		return "arch"
	}
	if strings.Contains(content, "ID=alpine") {
		return "alpine"
	}
	return "unknown"
}

// HasNftables 检查系统是否使用 nftables
func HasNftables() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// 检查 nftables 是否可用
	_, err := exec.LookPath("nft")
	return err == nil
}

// HasIptables 检查系统是否使用 iptables
func HasIptables() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	_, err := exec.LookPath("iptables")
	return err == nil
}

// SetupLinuxFirewall 配置 Linux 防火墙规则
func SetupLinuxFirewall(ifaceName string, enable bool) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if HasNftables() {
		return setupNftables(ifaceName, enable)
	}
	if HasIptables() {
		return setupIptables(ifaceName, enable)
	}

	return fmt.Errorf("no firewall tool found (nftables/iptables)")
}

func setupNftables(ifaceName string, enable bool) error {
	if enable {
		// 创建表和链
		cmd := exec.Command("nft", "add", "table", "inet", "zem")
		cmd.Run() // 忽略错误（可能已存在）

		cmd = exec.Command("nft", "add", "chain", "inet", "zem", "output", "{ type filter hook output priority 0 ; }")
		cmd.Run()

		// 允许 TUN 设备流量
		cmd = exec.Command("nft", "add", "rule", "inet", "zem", "output", "oifname", ifaceName, "accept")
		return cmd.Run()
	} else {
		// 清理规则
		cmd := exec.Command("nft", "delete", "table", "inet", "zem")
		return cmd.Run()
	}
}

func setupIptables(ifaceName string, enable bool) error {
	if enable {
		cmd := exec.Command("iptables", "-I", "OUTPUT", "-o", ifaceName, "-j", "ACCEPT")
		return cmd.Run()
	} else {
		cmd := exec.Command("iptables", "-D", "OUTPUT", "-o", ifaceName, "-j", "ACCEPT")
		return cmd.Run()
	}
}

// CheckTUNSupport 检查系统是否支持 TUN 设备
func CheckTUNSupport() error {
	if runtime.GOOS != "linux" {
		return nil
	}

	// 检查 /dev/net/tun
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("TUN 设备不可用: /dev/net/tun 不存在，请加载 tun 模块: sudo modprobe tun")
	}

	return nil
}

// InstallTUNModule 尝试安装 TUN 模块（Debian/Ubuntu）
func InstallTUNModule() error {
	if runtime.GOOS != "linux" {
		return nil
	}

	distro := GetLinuxDistro()

	switch distro {
	case "debian":
		cmd := exec.Command("apt-get", "update")
		cmd.Run()
		cmd = exec.Command("apt-get", "install", "-y", "tun-utils")
		return cmd.Run()
	case "redhat":
		cmd := exec.Command("yum", "install", "-y", "tunctl")
		return cmd.Run()
	case "arch":
		cmd := exec.Command("pacman", "-S", "--noconfirm", "tun-utils")
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported distro for auto-install: %s", distro)
	}
}
