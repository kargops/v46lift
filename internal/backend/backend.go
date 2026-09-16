package backend

import "context"

type Backend interface {
	Start(context.Context) error
	Stop(context.Context) error
}
