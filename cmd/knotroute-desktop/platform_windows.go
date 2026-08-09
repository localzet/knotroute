//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/localzet/knotroute/internal/clientruntime"
)

type proxyBackup struct {
	Enabled       bool   `json:"enabled"`
	HadAutoConfig bool   `json:"had_auto_config"`
	AutoConfigURL string `json:"auto_config_url,omitempty"`
}

const internetSettingsKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
const startupKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func enableBrowserIntegration(rt *clientruntime.Runtime, dataDir string) error {
	if err := rt.InstallCA(); err != nil {
		return err
	}
	current, exists := regQueryAt(internetSettingsKey, "AutoConfigURL")
	state := proxyBackup{Enabled: false, HadAutoConfig: exists, AutoConfigURL: current}
	path := filepath.Join(dataDir, "proxy-state.json")
	if err := writeProxyState(path, state); err != nil {
		_ = rt.UninstallCA()
		return err
	}
	const pac = "http://127.0.0.1:8484/proxy.pac"
	if err := regSetAt(internetSettingsKey, "AutoConfigURL", pac); err != nil {
		_ = rt.UninstallCA()
		return err
	}
	state.Enabled = true
	if err := writeProxyState(path, state); err != nil {
		if state.HadAutoConfig {
			_ = regSetAt(internetSettingsKey, "AutoConfigURL", state.AutoConfigURL)
		} else {
			_ = regDeleteAt(internetSettingsKey, "AutoConfigURL")
		}
		_ = rt.UninstallCA()
		return err
	}
	refreshInternetSettings()
	return nil
}

func disableBrowserIntegration(rt *clientruntime.Runtime, dataDir string) error {
	path := filepath.Join(dataDir, "proxy-state.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		_ = rt.UninstallCA()
		return nil
	}
	if err != nil {
		return err
	}
	var state proxyBackup
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if state.HadAutoConfig {
		if err := regSetAt(internetSettingsKey, "AutoConfigURL", state.AutoConfigURL); err != nil {
			return err
		}
	} else if err := regDeleteAt(internetSettingsKey, "AutoConfigURL"); err != nil {
		return err
	}
	_ = rt.UninstallCA()
	state.Enabled = false
	_ = writeProxyState(path, state)
	refreshInternetSettings()
	return nil
}

func browserIntegrationEnabled(dataDir string) bool {
	raw, err := os.ReadFile(filepath.Join(dataDir, "proxy-state.json"))
	if err != nil {
		return false
	}
	var state proxyBackup
	if json.Unmarshal(raw, &state) != nil || !state.Enabled {
		return false
	}
	current, ok := regQueryAt(internetSettingsKey, "AutoConfigURL")
	return ok && strings.EqualFold(strings.TrimSpace(current), "http://127.0.0.1:8484/proxy.pac")
}

func writeProxyState(path string, state proxyBackup) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func startupEnabled() bool { v, ok := regQueryAt(startupKey, "KnotRoute"); return ok && v != "" }
func setStartup(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if enabled {
		return regSetAt(startupKey, "KnotRoute", `"`+exe+`"`)
	}
	return regDeleteAt(startupKey, "KnotRoute")
}

func shellOpenPath(path string) error {
	cmd := exec.Command("explorer.exe", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func regQueryAt(key, name string) (string, bool) {
	cmd := exec.Command("reg.exe", "query", key, "/v", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 3 && strings.EqualFold(f[0], name) {
			return strings.Join(f[2:], " "), true
		}
	}
	return "", false
}
func regSetAt(key, name, value string) error {
	cmd := exec.Command("reg.exe", "add", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registry: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
func regDeleteAt(key, name string) error {
	if _, ok := regQueryAt(key, name); !ok {
		return nil
	}
	cmd := exec.Command("reg.exe", "delete", key, "/v", name, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_, err := cmd.CombinedOutput()
	return err
}

var wininet = syscall.NewLazyDLL("wininet.dll")
var internetSetOptionW = wininet.NewProc("InternetSetOptionW")

func refreshInternetSettings() {
	// INTERNET_OPTION_SETTINGS_CHANGED, INTERNET_OPTION_REFRESH, INTERNET_OPTION_PROXY_SETTINGS_CHANGED.
	for _, option := range []uintptr{39, 37, 95} {
		internetSetOptionW.Call(0, option, 0, 0)
	}
}
