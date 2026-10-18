package system

import (
	"errors"
	"fmt"
	"runtime"
)

var ErrUnsupportedOS = errors.New("unsupported operating system")

func EnsureCurrentOSSupported() error {
	return EnsureSupportedOS(runtime.GOOS)
}

func EnsureSupportedOS(goos string) error {
	if IsSupportedOS(goos) {
		return nil
	}

	return fmt.Errorf("%w: only macOS on Apple Silicon is supported (detected %s)", ErrUnsupportedOS, goos)
}

func EnsureSupportedPlatform(profile PlatformProfile) error {
	return EnsureSupportedOS(profile.OS)
}
