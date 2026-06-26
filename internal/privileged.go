package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// OgBin returns the absolute path to this binary.
func OgBin() (string, error) {
	p, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// Privileged runs og itself with sudo for internal privileged commands.
// SUDO_USER and OG_HOME are passed as VAR=value on the command line (sudo-rs
// supports this with the SETENV tag), so the root process can resolve the
// real user's config without relying on the environment (which sudo-rs resets).
func Privileged(args ...string) (string, error) {
	bin, err := OgBin()
	if err != nil {
		return "", err
	}
	if os.Geteuid() == 0 {
		out, err := exec.Command(bin, args...).CombinedOutput()
		return string(out), err
	}
	user := os.Getenv("USER")
	ogHome := os.Getenv("OG_HOME")
	sudoArgs := []string{"-u", "root"}
	if user != "" {
		sudoArgs = append(sudoArgs, "SUDO_USER="+user)
	}
	if ogHome != "" {
		sudoArgs = append(sudoArgs, "OG_HOME="+ogHome)
	}
	sudoArgs = append(sudoArgs, bin)
	sudoArgs = append(sudoArgs, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// MountWorkspacePrivileged mounts the overlay for a workspace via sudo.
func MountWorkspacePrivileged(name string) error {
	out, err := Privileged("--internal-mount", name)
	if err != nil {
		return fmt.Errorf("privileged mount: %s", out)
	}
	return nil
}

// UmountWorkspacePrivileged unmounts the overlay via sudo.
func UmountWorkspacePrivileged(name string) error {
	out, err := Privileged("--internal-umount", name)
	if err != nil {
		return fmt.Errorf("privileged umount: %s", out)
	}
	return nil
}

// LockBasePrivileged locks the base via sudo.
func LockBasePrivileged() error {
	out, err := Privileged("--internal-lock")
	if err != nil {
		return fmt.Errorf("privileged lock: %s", out)
	}
	return nil
}

// UnlockBasePrivileged unlocks the base via sudo.
func UnlockBasePrivileged() error {
	out, err := Privileged("--internal-unlock")
	if err != nil {
		return fmt.Errorf("privileged unlock: %s", out)
	}
	return nil
}

// PurgeWorkspacePrivileged deletes store dir + worktree git-dir via sudo.
func PurgeWorkspacePrivileged(name string) error {
	out, err := Privileged("--internal-purge", name)
	if err != nil {
		return fmt.Errorf("privileged purge: %s", out)
	}
	return nil
}
