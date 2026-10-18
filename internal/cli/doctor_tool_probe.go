package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const doctorToolProbeTimeout = 5 * time.Second

// Version commands are non-interactive probes, not installation or repair.
// Discard output rather than buffering potentially unbounded subprocess data.
func probeDoctorTool(parent context.Context, tool, path string) error {
	ctx, cancel := context.WithTimeout(parent, doctorToolProbeTimeout)
	defer cancel()
	arg := "--version"
	if tool == "engram" {
		arg = "version"
	}
	cmd, err := doctorToolProbeCommand(ctx, path, arg)
	if err != nil {
		return err
	}
	// A child that inherits output handles must not keep Wait blocked forever.
	cmd.WaitDelay = 100 * time.Millisecond
	err = runDoctorProbeCommand(ctx, cmd)
	if parent.Err() != nil {
		return parent.Err()
	}
	if ctx.Err() != nil {
		return fmt.Errorf("did not complete within %s: %w", doctorToolProbeTimeout, ctx.Err())
	}
	return err
}

// Keep descendants in the probe's process group/job and release the whole tree
// before returning, including when a launcher exits before its child does.
func runDoctorProbeCommand(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	terminate, err := startDoctorProbeProcessTree(cmd)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	return errors.Join(err, terminate())
}
