package network

import (
	"context"

	"github.com/kargops/v46lift/internal/config"
)

type Manager interface {
	Up(context.Context, config.NetworkConfig) error
	Down(context.Context) error
}

func New() Manager {
	return newPlatformManager()
}
