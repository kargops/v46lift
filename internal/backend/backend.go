package backend

import "context"

type Backend interface {
	Start(context.Context) error
	Wait(context.Context) error
	Stop(context.Context) error
}
