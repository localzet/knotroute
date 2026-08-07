//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	serviceWin32OwnProcess = 0x10
	serviceStopped         = 0x1
	serviceStartPending    = 0x2
	serviceStopPending     = 0x3
	serviceRunning         = 0x4
	serviceAcceptStop      = 0x1
	serviceAcceptShutdown  = 0x4
	serviceControlStop     = 0x1
	serviceControlShutdown = 0x5
	errorServiceSpecific   = 1066
)

var (
	advapi32                          = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcherW   = advapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandlerExW = advapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus              = advapi32.NewProc("SetServiceStatus")

	serviceState *windowsService
)

type serviceTableEntry struct {
	Name *uint16
	Proc uintptr
}

type serviceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}

type serviceConfig struct {
	Dashboard string `json:"dashboard"`
}

type windowsService struct {
	mu           sync.Mutex
	statusHandle uintptr
	stop         chan struct{}
	stopOnce     sync.Once
	configPath   string
	daemonPath   string
	cmd          *exec.Cmd
	logFile      *os.File
}

func runService() {
	state, err := newWindowsService()
	if err != nil {
		return
	}
	serviceState = state
	name, _ := syscall.UTF16PtrFromString("KnotRoute")
	table := []serviceTableEntry{{Name: name, Proc: syscall.NewCallback(serviceMain)}, {}}
	procStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
}

func newWindowsService() (*windowsService, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return nil, errors.New("ProgramData is not set")
	}
	configPath := filepath.Join(programData, "KnotRoute", "knotroute.json")
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			i++
		}
	}
	return &windowsService{stop: make(chan struct{}), configPath: configPath, daemonPath: filepath.Join(filepath.Dir(exe), "knotroute.exe")}, nil
}

func serviceMain(argc uint32, argv uintptr) uintptr {
	s := serviceState
	name, _ := syscall.UTF16PtrFromString("KnotRoute")
	handle, _, _ := procRegisterServiceCtrlHandlerExW.Call(uintptr(unsafe.Pointer(name)), syscall.NewCallback(serviceHandler), 0)
	if handle == 0 {
		return 0
	}
	s.statusHandle = handle
	s.setStatus(serviceStartPending, 0, 1, 15000)
	if err := s.ensureConfig(); err != nil {
		s.setStopped(err)
		return 0
	}
	if err := s.startDaemon(); err != nil {
		s.setStopped(err)
		return 0
	}
	s.setStatus(serviceRunning, serviceAcceptStop|serviceAcceptShutdown, 0, 0)
	exited := make(chan error, 1)
	go func() { exited <- s.cmd.Wait() }()
	select {
	case <-s.stop:
		s.setStatus(serviceStopPending, 0, 1, 10000)
		s.stopDaemon()
		select {
		case <-exited:
		case <-time.After(8 * time.Second):
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
			<-exited
		}
		s.setStatus(serviceStopped, 0, 0, 0)
	case err := <-exited:
		if err != nil {
			s.setStopped(err)
		} else {
			s.setStatus(serviceStopped, 0, 0, 0)
		}
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
	return 0
}

func serviceHandler(control, eventType uint32, eventData, context uintptr) uintptr {
	if control == serviceControlStop || control == serviceControlShutdown {
		serviceState.stopOnce.Do(func() { close(serviceState.stop) })
	}
	return 0
}

func (s *windowsService) setStatus(state, accepted, checkpoint, waitHint uint32) {
	status := serviceStatus{ServiceType: serviceWin32OwnProcess, CurrentState: state, ControlsAccepted: accepted, CheckPoint: checkpoint, WaitHint: waitHint}
	procSetServiceStatus.Call(s.statusHandle, uintptr(unsafe.Pointer(&status)))
}

func (s *windowsService) setStopped(err error) {
	status := serviceStatus{ServiceType: serviceWin32OwnProcess, CurrentState: serviceStopped}
	if err != nil {
		status.Win32ExitCode = errorServiceSpecific
		status.ServiceSpecificExitCode = 1
	}
	procSetServiceStatus.Call(s.statusHandle, uintptr(unsafe.Pointer(&status)))
}

func (s *windowsService) ensureConfig() error {
	if _, err := os.Stat(s.daemonPath); err != nil {
		return fmt.Errorf("knotroute.exe not found: %w", err)
	}
	if _, err := os.Stat(s.configPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o700); err != nil {
		return err
	}
	cmd := exec.Command(s.daemonPath, "init", "--config", s.configPath, "--listen", "0.0.0.0:7447", "--dashboard", "127.0.0.1:8484")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("initialize: %v: %s", err, bytes.TrimSpace(output))
	}
	return nil
}

func (s *windowsService) startDaemon() error {
	logPath := filepath.Join(filepath.Dir(s.configPath), "knotroute-service.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(s.daemonPath, "run", "--config", s.configPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000200}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	s.cmd = cmd
	s.logFile = logFile
	return nil
}

func (s *windowsService) stopDaemon() {
	url := s.dashboardURL()
	if url != "" {
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest(http.MethodPost, url+"/api/shutdown", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		if response, err := client.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return
		}
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

func (s *windowsService) dashboardURL() string {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return ""
	}
	var cfg serviceConfig
	if json.Unmarshal(data, &cfg) != nil || cfg.Dashboard == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(cfg.Dashboard)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
