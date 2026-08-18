package shared

import "runtime"

const (
	Darwin  = iota // 0
	Linux          // 1
	Windows        // 2
	Unknown        // 3
)

// simple check for OS
func CheckOS() int {
	switch os := runtime.GOOS; os {
	case "darwin":
		return Darwin
	case "linux":
		return Linux
	case "windows":
		return Windows
	default:
		return Unknown
	}
}
