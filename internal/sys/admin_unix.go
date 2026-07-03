//go:build !windows

package sys

func checkWindowsAdmin() bool {
	return false
}
