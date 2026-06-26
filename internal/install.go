package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallSudoers creates /etc/sudoers.d/og with NOPASSWD rules for
// the internal privileged commands. Paths are derived from config
// inside the binary, not from command-line args.
func InstallSudoers() error {
	bin, err := OgBin()
	if err != nil {
		return err
	}
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("cannot determine current user")
	}

	// Allow passwordless internal commands with SETENV (so SUDO_USER can be passed).
	content := fmt.Sprintf(`%s ALL=(root) NOPASSWD:SETENV: %s --internal-mount *
%s ALL=(root) NOPASSWD:SETENV: %s --internal-umount *
%s ALL=(root) NOPASSWD:SETENV: %s --internal-purge *
%s ALL=(root) NOPASSWD:SETENV: %s --internal-lock
%s ALL=(root) NOPASSWD:SETENV: %s --internal-unlock
`, user, bin, user, bin, user, bin, user, bin, user, bin)

	tmp, err := os.CreateTemp("", "og-sudoers-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	tmp.Close()

	// Write via sudo.
	if out, err := exec.Command("sudo", "cp", tmp.Name(), "/etc/sudoers.d/og").CombinedOutput(); err != nil {
		return fmt.Errorf("sudo cp sudoers: %s", string(out))
	}
	if out, err := exec.Command("sudo", "chmod", "0440", "/etc/sudoers.d/og").CombinedOutput(); err != nil {
		return fmt.Errorf("sudo chmod sudoers: %s", string(out))
	}
	// Validate.
	if out, err := exec.Command("sudo", "visudo", "-cf", "/etc/sudoers.d/og").CombinedOutput(); err != nil {
		return fmt.Errorf("visudo check: %s", string(out))
	}
	return nil
}

// InstallSystemd creates a user systemd service for auto-resume on login.
func InstallSystemd() error {
	bin, err := OgBin()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return err
	}
	unitPath := filepath.Join(unitDir, "og-resume.service")

	content := fmt.Sprintf(`[Unit]
Description=og: resume overlay workspaces after reboot
After=default.target

[Service]
Type=oneshot
ExecStart=%s resume
RemainAfterExit=yes

[Install]
WantedBy=default.target
`, bin)

	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return err
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s", string(out))
	}
	_ = exec.Command("systemctl", "--user", "enable", "og-resume.service").Run()

	// Enable linger so user services start at boot.
	user := os.Getenv("USER")
	if user != "" {
		_ = exec.Command("sudo", "loginctl", "enable-linger", user).Run()
	}
	return nil
}

// AddExclude adds /.og/ to .git/info/exclude to prevent git tracking.
func AddExclude(base string) error {
	excludePath := filepath.Join(base, ".git", "info", "exclude")
	entry := "/.og/\n"

	// Read existing content.
	existing, _ := os.ReadFile(excludePath)
	for _, line := range splitLines(string(existing)) {
		if line == "/.og/" {
			return nil // already excluded
		}
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
