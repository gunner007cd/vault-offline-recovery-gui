//go:build !windows

package main

import "errors"

func availableSpace(string) (int64, error) {
	return 0, errors.New("volume space query is only implemented for Windows")
}
