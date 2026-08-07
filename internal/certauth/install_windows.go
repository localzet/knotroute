//go:build windows

package certauth

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func InstallUserRoot(a *Authority) error {
	cmd := exec.Command("certutil.exe", "-user", "-addstore", "Root", a.RootPath())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil addstore: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
func UninstallUserRoot(a *Authority) error {
	cmd := exec.Command("certutil.exe", "-user", "-delstore", "Root", a.Fingerprint())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil delstore: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
