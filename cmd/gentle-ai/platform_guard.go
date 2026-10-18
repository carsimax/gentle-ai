package main

import "fmt"

// ensureForkPlatform rejects every platform except macOS on Apple Silicon.
// This fork drops Windows, Linux and Intel macOS support.
func ensureForkPlatform(goos, goarch string) error {
	if goos == "darwin" && goarch == "arm64" {
		return nil
	}
	return fmt.Errorf("unsupported platform %s/%s: this build supports only macOS on Apple Silicon (darwin/arm64)", goos, goarch)
}
