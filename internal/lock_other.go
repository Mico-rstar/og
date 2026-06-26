//go:build !linux

package internal

import "os"

func flock(*os.File) error       { return nil }
func flockUnlock(*os.File) error { return nil }
