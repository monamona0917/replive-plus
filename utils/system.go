package utils

import "runtime"

const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSMac     = "darwin"
)

func GetOS() string {
	return runtime.GOOS
}

func IsWindows() bool {
	return GetOS() == OSWindows
}
