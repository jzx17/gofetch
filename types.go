package gofetch

import (
	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
)

type Request = core.Request
type Response = core.Response
type AsyncResponse = core.AsyncResponse
type SizeConfig = core.SizeConfig
type StreamOption = core.StreamOption

var NewRequest = core.NewRequest
var DefaultSizeConfig = core.DefaultSizeConfig
var WithBufferSize = core.WithBufferSize

type RoundTripFunc = core.RoundTripFunc
type TLSTransport = core.TLSTransport
type ConfigurableMiddleware = middlewares.ConfigurableMiddleware
type MiddlewareIdentifier = middlewares.MiddlewareIdentifier
type Middleware = middlewares.Middleware

var NewTLSTransport = core.NewTLSTransport
var CreateMiddleware = middlewares.CreateMiddleware
var ChainMiddlewares = middlewares.ChainMiddlewares
var SizeValidationMiddleware = middlewares.SizeValidationMiddleware
var RetryMiddleware = middlewares.RetryMiddleware
var RateLimitMiddleware = middlewares.RateLimitMiddleware
var LoggingMiddleware = middlewares.LoggingMiddleware

type SizeError = middlewares.SizeError
type RetryError = middlewares.RetryError
type TimeoutError = middlewares.TimeoutError
type RateLimitExceededError = middlewares.RateLimitExceededError
type RateLimitOptions = middlewares.RateLimitOptions
type LoggingOptions = middlewares.LoggingOptions
type LogLevel = middlewares.LogLevel
type LogFormat = middlewares.LogFormat
