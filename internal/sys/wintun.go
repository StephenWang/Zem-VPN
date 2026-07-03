package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// CheckAdmin 检查是否以管理员/root权限运行
func CheckAdmin() bool {
	switch runtime.GOOS {
	case "windows":
		return checkWindowsAdmin()
	case "darwin", "linux":
		return os.Getuid() == 0
	default:
		return false
	}
}

// EnsureAdmin 确保以管理员权限运行，否则返回错误
func EnsureAdmin() error {
	if CheckAdmin() {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("请以管理员身份运行此程序")
	case "darwin":
		return fmt.Errorf("请使用 sudo 运行此程序")
	case "linux":
		return fmt.Errorf("请使用 sudo 或以 root 运行此程序")
	default:
		return fmt.Errorf("需要提升权限")
	}
}

// ExtractWintun 释放 wintun.dll 到程序目录
// 如果 wintun.dll 已嵌入，从嵌入资源释放
// 否则尝试从系统 PATH 查找
func ExtractWintun() (string, error) {
	if runtime.GOOS != "windows" {
		return "", nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	dllDir := filepath.Dir(exePath)
	dllPath := filepath.Join(dllDir, "wintun.dll")

	// 如果已存在，直接返回
	if _, err := os.Stat(dllPath); err == nil {
		return dllPath, nil
	}

	// 尝试从嵌入资源释放（如果编译时嵌入了 wintun.dll）
	if embeddedWintun != nil {
		if err := os.WriteFile(dllPath, embeddedWintun, 0644); err != nil {
			return "", fmt.Errorf("释放 wintun.dll 失败: %w", err)
		}
		return dllPath, nil
	}

	// 尝试从系统 PATH 查找
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		for _, dir := range filepath.SplitList(pathEnv) {
			tryPath := filepath.Join(dir, "wintun.dll")
			if _, err := os.Stat(tryPath); err == nil {
				return tryPath, nil
			}
		}
	}

	return "", fmt.Errorf("wintun.dll 未找到，请下载并放入程序目录: https://www.wintun.net")
}

// embeddedWintun 用于嵌入 wintun.dll
// 使用 go:embed 指令在编译时嵌入
// 如果未嵌入，此变量为 nil
var embeddedWintun []byte

// SetEmbeddedWintun 设置嵌入的 wintun.dll 数据
// 在 init() 或 main() 中调用，如果使用外部嵌入方式
func SetEmbeddedWintun(data []byte) {
	embeddedWintun = data
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
