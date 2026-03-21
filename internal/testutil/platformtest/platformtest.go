package platformtest

import "runtime"

func SupportsExactFileModes() bool {
	return runtime.GOOS != "windows"
}

func SupportsPermissionChecks() bool {
	return runtime.GOOS != "windows"
}
