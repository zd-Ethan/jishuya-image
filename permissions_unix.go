//go:build !windows

package main

import "os"

// protectPath 将配置目录和文件限制为当前用户访问。
func protectPath(path string, directory bool) error {
	mode := os.FileMode(0600)
	if directory {
		mode = 0700
	}
	return os.Chmod(path, mode)
}
