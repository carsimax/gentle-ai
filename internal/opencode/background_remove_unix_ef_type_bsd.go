package opencode

import (
	"errors"

	"golang.org/x/sys/unix"
)

func isManagedLauncherEFTYPE(err error) bool {
	return errors.Is(err, unix.EFTYPE)
}
