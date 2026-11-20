//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// This queries the filesystem. It never writes to destination or backup data.
func availableSpace(dest string) (int64, error) {
	dir, err := existingDestinationAncestor(dest)
	if err != nil {
		return 0, err
	}
	var available, total, totalFree uint64
	p := syscall.StringToUTF16Ptr(dir)
	ok, _, e := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
	if ok == 0 {
		return 0, fmt.Errorf("GetDiskFreeSpaceExW: %w", e)
	}
	if available > 9223372036854775807 {
		return 9223372036854775807, nil
	}
	return int64(available), nil
}
