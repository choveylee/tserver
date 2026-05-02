package tmiddleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/choveylee/tlimiter"
)

// LimiterConfig defines the behavior of [LimiterMiddleware], including the underlying
// limiter, error handling, limit-exceeded handling, key extraction, and optional key
// exclusion.
type LimiterConfig struct {
	Limiter *tlimiter.Limiter

	ErrorHandler        ErrorHandler
	LimitReachedHandler LimitReachedHandler

	KeyGetter KeyGetter

	ExcludedKey func(string) bool
}

// Handle applies the configured rate limit to a single request. It may abort the handler
// chain on limiter errors or when the limit has been exceeded, or continue when the
// request is allowed.
func (config *LimiterConfig) Handle(c *gin.Context) {
	key := config.KeyGetter(c)

	if config.ExcludedKey != nil && config.ExcludedKey(key) {
		c.Next()

		return
	}

	context, err := config.Limiter.Get(c, key)
	if err != nil {
		config.ErrorHandler(c, err)

		c.Abort()

		return
	}

	c.Header("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))

	if context.Reached {
		config.LimitReachedHandler(c)

		c.Abort()

		return
	}

	c.Next()
}

// LimiterConfigOptionInterface is implemented by functional options for
// [LimiterMiddleware].
type LimiterConfigOptionInterface interface {
	apply(*LimiterConfig)
}

// LimiterConfigOption is a functional option that mutates a [LimiterConfig] when applied.
type LimiterConfigOption func(*LimiterConfig)

func (option LimiterConfigOption) apply(config *LimiterConfig) {
	option(config)
}

// ErrorHandler handles errors returned by the limiter while resolving state for the
// current request key.
type ErrorHandler func(c *gin.Context, err error)

// WithErrorHandler configures [LimiterMiddleware] to use the provided [ErrorHandler].
func WithErrorHandler(handler ErrorHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ErrorHandler = handler
	})
}

// DefaultErrorHandler is the default [ErrorHandler] used by [LimiterMiddleware].
func DefaultErrorHandler(c *gin.Context, err error) {
	panic(err)
}

// LimitReachedHandler is invoked when the rate limit for the current key has been exceeded.
type LimitReachedHandler func(c *gin.Context)

// WithLimitReachedHandler configures [LimiterMiddleware] to use the provided
// [LimitReachedHandler].
func WithLimitReachedHandler(handler LimitReachedHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.LimitReachedHandler = handler
	})
}

// DefaultLimitReachedHandler is the default [LimitReachedHandler] used by
// [LimiterMiddleware].
func DefaultLimitReachedHandler(c *gin.Context) {
	c.String(http.StatusTooManyRequests, "limit exceeded")
}

// KeyGetter derives the rate-limit key from the current Gin context.
type KeyGetter func(c *gin.Context) string

// WithKeyGetter configures [LimiterMiddleware] to use the provided [KeyGetter].
func WithKeyGetter(handler KeyGetter) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.KeyGetter = handler
	})
}

// DefaultKeyGetter is the default [KeyGetter] used by [LimiterMiddleware]. It returns
// the client IP address.
func DefaultKeyGetter(c *gin.Context) string {
	return c.ClientIP()
}

// WithExcludedKey configures [LimiterMiddleware] to skip rate limiting for keys for which
// the provided predicate returns true.
func WithExcludedKey(handler func(string) bool) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ExcludedKey = handler
	})
}

// LimiterMiddleware returns Gin middleware that enforces rate limits using limiter.
// Additional behavior may be customized with [WithErrorHandler],
// [WithLimitReachedHandler], [WithKeyGetter], and [WithExcludedKey].
func LimiterMiddleware(limiter *tlimiter.Limiter, options ...LimiterConfigOptionInterface) gin.HandlerFunc {
	limiterConfig := &LimiterConfig{
		Limiter: limiter,

		ErrorHandler:        DefaultErrorHandler,
		LimitReachedHandler: DefaultLimitReachedHandler,

		KeyGetter: DefaultKeyGetter,

		ExcludedKey: nil,
	}

	for _, option := range options {
		option.apply(limiterConfig)
	}

	return func(ctx *gin.Context) {
		limiterConfig.Handle(ctx)
	}
}
