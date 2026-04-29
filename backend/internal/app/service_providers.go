package app

import (
	"errors"
	"strings"
)

type serviceProvider interface {
	Name() string
}

type systemdProvider struct{}

func (p systemdProvider) Name() string { return "systemd" }

type supervisorProvider struct{}

func (p supervisorProvider) Name() string { return "supervisor" }

type dockerComposeProvider struct{}

func (p dockerComposeProvider) Name() string { return "docker-compose" }

func resolveServiceProvider(raw string) (serviceProvider, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "systemd":
		return systemdProvider{}, nil
	case "supervisor":
		return supervisorProvider{}, nil
	case "docker-compose", "docker_compose", "compose":
		return dockerComposeProvider{}, nil
	default:
		return nil, errors.New("unsupported provider")
	}
}
