//go:build windows

package cli

import "os"

func secretFileOwner(os.FileInfo) error { return nil }
