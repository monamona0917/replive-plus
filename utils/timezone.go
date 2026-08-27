package utils

import "time"

// LocalLocation 返回运行电脑当前配置的系统时区。
// Go 会从操作系统初始化 time.Local。
func LocalLocation() *time.Location {
	return time.Local
}
