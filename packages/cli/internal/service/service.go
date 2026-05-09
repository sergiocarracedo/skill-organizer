package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	servicepkg "github.com/kardianos/service"

	loggingpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/logging"
	watchpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/watch"
)

const (
	serviceName        = "skill-organizer"
	serviceDisplayName = "Skill Organizer"
	serviceDescription = "Watches registered skill-organizer projects and keeps them synchronized"
)

type Program struct {
	RegistryPath string
	runner       *watchpkg.Runner
	logger       loggingpkg.Logger
}

func NewProgram(registryPath string) *Program {
	return &Program{RegistryPath: registryPath}
}

func New(registryPath string) (servicepkg.Service, error) {
	program := NewProgram(registryPath)
	config := &servicepkg.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		Arguments:   []string{"watch"},
		Option: servicepkg.KeyValue{
			"UserService": true,
		},
	}

	serviceInstance, err := servicepkg.New(program, config)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}

	return serviceInstance, nil
}

func (p *Program) Start(s servicepkg.Service) error {
	p.logger = loggingpkg.LoadForRegistry(p.RegistryPath, s)
	p.logger.Infof("service starting")

	runner, err := watchpkg.New(p.RegistryPath, p.logger)
	if err != nil {
		p.logger.Errorf("failed to create watch runner: %v", err)
		return err
	}
	p.runner = runner

	go func() {
		if err := p.runner.Run(); err != nil {
			p.logger.Errorf("watch runner stopped with error: %v", err)
			return
		}
		p.logger.Infof("watch runner stopped")
	}()

	return nil
}

func (p *Program) Stop(_ servicepkg.Service) error {
	if p.runner == nil {
		if p.logger != nil {
			p.logger.Infof("service stop requested with no active runner")
		}
		return nil
	}
	if p.logger != nil {
		p.logger.Infof("service stopping")
	}
	return p.runner.Close()
}

func Control(registryPath string, action string) (string, error) {
	if runtime.GOOS == "linux" {
		return controlLinuxUserSystemd(action)
	}

	svc, err := New(registryPath)
	if err != nil {
		return "", err
	}

	switch action {
	case "install":
		if err := svc.Install(); err != nil {
			return "", fmt.Errorf("install service: %w", err)
		}
		return "installed", nil
	case "start":
		if err := svc.Start(); err != nil {
			return "", fmt.Errorf("start service: %w", err)
		}
		return "started", nil
	case "stop":
		if err := svc.Stop(); err != nil {
			return "", fmt.Errorf("stop service: %w", err)
		}
		return "stopped", nil
	case "restart":
		if err := svc.Restart(); err != nil {
			return "", fmt.Errorf("restart service: %w", err)
		}
		return "restarted", nil
	case "uninstall":
		if err := svc.Uninstall(); err != nil {
			return "", fmt.Errorf("uninstall service: %w", err)
		}
		return "uninstalled", nil
	case "status":
		status, err := svc.Status()
		if err != nil {
			return "", fmt.Errorf("service status: %w", err)
		}
		return formatStatus(status), nil
	default:
		return "", fmt.Errorf("unsupported service action %q", action)
	}
}

func WaitForStopDelay() {
	time.Sleep(100 * time.Millisecond)
}

func formatStatus(status servicepkg.Status) string {
	switch status {
	case servicepkg.StatusRunning:
		return "running"
	case servicepkg.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func controlLinuxUserSystemd(action string) (string, error) {
	unitPath, err := userUnitPath()
	if err != nil {
		return "", err
	}

	switch action {
	case "install":
		if err := installUserUnit(unitPath); err != nil {
			return "", fmt.Errorf("install service: %w", err)
		}
		return "installed", nil
	case "start":
		if err := runUserSystemctl("start", serviceName+".service"); err != nil {
			return "", fmt.Errorf("start service: %w", err)
		}
		return "started", nil
	case "stop":
		if err := runUserSystemctl("stop", serviceName+".service"); err != nil {
			return "", fmt.Errorf("stop service: %w", err)
		}
		return "stopped", nil
	case "restart":
		if err := runUserSystemctl("restart", serviceName+".service"); err != nil {
			return "", fmt.Errorf("restart service: %w", err)
		}
		return "restarted", nil
	case "status":
		status, err := userSystemdStatus()
		if err != nil {
			return "", fmt.Errorf("service status: %w", err)
		}
		return status, nil
	case "uninstall":
		_ = runUserSystemctl("stop", serviceName+".service")
		_ = runUserSystemctl("disable", serviceName+".service")
		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("remove service unit: %w", err)
		}
		if err := runUserSystemctl("daemon-reload"); err != nil {
			return "", fmt.Errorf("reload user systemd: %w", err)
		}
		return "uninstalled", nil
	default:
		return "", fmt.Errorf("unsupported service action %q", action)
	}
}

func installUserUnit(unitPath string) error {
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("create user systemd directory: %w", err)
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("resolve absolute executable: %w", err)
	}

	unit := fmt.Sprintf(`[Unit]
Description=%s

[Service]
Type=simple
ExecStart=%q watch
Restart=always
RestartSec=2

[Install]
WantedBy=default.target
`, serviceDescription, executable)

	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write user service unit: %w", err)
	}

	if err := runUserSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("reload user systemd: %w", err)
	}
	if err := runUserSystemctl("enable", serviceName+".service"); err != nil {
		return fmt.Errorf("enable user service: %w", err)
	}

	return nil
}

func userUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", serviceName+".service"), nil
}

func runUserSystemctl(args ...string) error {
	command := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func userSystemdStatus() (string, error) {
	command := exec.Command("systemctl", "--user", "is-active", serviceName+".service")
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "inactive" {
			return "stopped", nil
		}
		if text == "failed" {
			return "failed", nil
		}
		if text == "unknown" || text == "" {
			return "unknown", nil
		}
		return "", fmt.Errorf("%s", text)
	}

	if text == "active" {
		return "running", nil
	}
	return text, nil
}
