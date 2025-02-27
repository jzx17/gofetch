package middlewares

import (
	"github.com/jzx17/gofetch/core"
)

// MiddlewareIdentifier represents metadata about a middleware
type MiddlewareIdentifier struct {
	Name    string      // Name of the middleware type
	Options interface{} // Configuration options for the middleware
}

// ConfigurableMiddleware is an interface that middleware can implement to be configurable
type ConfigurableMiddleware interface {
	GetIdentifier() MiddlewareIdentifier
	Wrap(next core.RoundTripFunc) core.RoundTripFunc
}

var _ ConfigurableMiddleware = (*BaseMiddleware)(nil)

type BaseMiddleware struct {
	Identifier MiddlewareIdentifier
	Wrapper    Middleware
}

func (m *BaseMiddleware) GetIdentifier() MiddlewareIdentifier {
	return m.Identifier
}

func (m *BaseMiddleware) Wrap(next core.RoundTripFunc) core.RoundTripFunc {
	return m.Wrapper(next)
}

// Middleware defines a function to wrap around a RoundTripFunc.
type Middleware func(next core.RoundTripFunc) core.RoundTripFunc

// CreateMiddleware creates a new configurable middleware
func CreateMiddleware(name string, options interface{}, wrapper Middleware) ConfigurableMiddleware {
	return &BaseMiddleware{
		Identifier: MiddlewareIdentifier{
			Name:    name,
			Options: options,
		},
		Wrapper: wrapper,
	}
}

// ChainMiddlewares applies a list of middleware functions around a final RoundTripFunc.
func ChainMiddlewares(final core.RoundTripFunc, mws ...ConfigurableMiddleware) core.RoundTripFunc {
	wrapped := final
	for i := len(mws) - 1; i >= 0; i-- {
		wrapped = mws[i].Wrap(wrapped)
	}
	return wrapped
}
