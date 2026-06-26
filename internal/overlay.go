package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

// isMountpoint returns true if path is a mount point.
func isMountpoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// MountOverlay mounts an overlay filesystem. Needs root — called via sudo.
func MountOverlay(base, upper, work, merged string) error {
	if isMountpoint(merged) {
		return nil // already mounted
	}
	if err := mkdirAll(merged, upper, work); err != nil {
		return err
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", base, upper, work)
	out, err := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, merged).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount overlay: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// UmountOverlay unmounts the overlay at merged. Needs root.
func UmountOverlay(merged string) error {
	if !isMountpoint(merged) {
		return nil // not mounted
	}
	if err := exec.Command("umount", merged).Run(); err != nil {
		// Fallback: lazy umount
		_ = exec.Command("umount", "-l", merged).Run()
	}
	return nil
}

// LockBase makes the base repo read-only via bind mount, but keeps .git writable.
// Needs root — called via sudo.
func LockBase(base string) error {
	// 1. Bind-mount base onto itself (makes it a mount point).
	if out, err := exec.Command("mount", "--bind", base, base).CombinedOutput(); err != nil {
		// If already a mount point, that's fine.
		if !strings.Contains(string(out), "already mounted") {
			return fmt.Errorf("bind mount base: %s", strings.TrimSpace(string(out)))
		}
	}
	// 2. Remount base read-only.
	if out, err := exec.Command("mount", "-o", "remount,ro", base).CombinedOutput(); err != nil {
		return fmt.Errorf("remount base ro: %s", strings.TrimSpace(string(out)))
	}
	// 3. Bind-mount .git onto itself.
	gitDir := base + "/.git"
	if out, err := exec.Command("mount", "--bind", gitDir, gitDir).CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "already mounted") {
			return fmt.Errorf("bind mount .git: %s", strings.TrimSpace(string(out)))
		}
	}
	// 4. Remount .git read-write.
	if out, err := exec.Command("mount", "-o", "remount,rw", gitDir).CombinedOutput(); err != nil {
		return fmt.Errorf("remount .git rw: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// UnlockBase reverses LockBase: umounts .git bind, then base bind.
func UnlockBase(base string) error {
	gitDir := base + "/.git"
	// Umount .git first (child before parent).
	_ = exec.Command("umount", gitDir).Run()
	// Then base.
	if err := exec.Command("umount", base).Run(); err != nil {
		_ = exec.Command("umount", "-l", base).Run()
	}
	return nil
}

// IsBaseLocked returns true if base is currently read-only bind-mounted.
func IsBaseLocked(base string) bool {
	// Check if base is a mount point and read-only.
	out, err := exec.Command("findmnt", "-n", "-o", "OPTIONS", base).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "ro")
}

func mkdirAll(dirs ...string) error {
	for _, d := range dirs {
		if err := exec.Command("mkdir", "-p", d).Run(); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}
