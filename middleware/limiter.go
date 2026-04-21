package tmiddleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/choveylee/tlimiter"
)

// LimiterConfig holds rate-limiter behavior for [LimiterMiddleware]: the underlying limiter,
// error and limit-exceeded handlers, key extraction, and optional key exclusion.
type LimiterConfig struct {
	Limiter *tlimiter.Limiter

	ErrorHandler        ErrorHandler
	LimitReachedHandler LimitReachedHandler

	KeyGetter KeyGetter

	ExcludedKey func(string) bool
}

// Handle applies the configured rate limit for a single request: it may abort the chain on
// errors, when the limit is reached, or continue to the next handler when allowed.
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

// LimiterConfigOptionInterface is implemented by functional options for [LimiterMiddleware].
type LimiterConfigOptionInterface interface {
	apply(*LimiterConfig)
}

// LimiterConfigOption is a functional option that mutates a [LimiterConfig] when applied.
type LimiterConfigOption func(*LimiterConfig)

func (option LimiterConfigOption) apply(config *LimiterConfig) {
	option(config)
}

// ErrorHandler is invoked when the limiter returns a non-nil error while resolving the key context.
type ErrorHandler func(c *gin.Context, err error)

// WithErrorHandler will configure the Middleware to use the given ErrorHandler.
func WithErrorHandler(handler ErrorHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ErrorHandler = handler
	})
}

// DefaultErrorHandler is the default ErrorHandler used by a new Middleware.
func DefaultErrorHandler(c *gin.Context, err error) {
	panic(err)
}

// LimitReachedHandler is invoked when the rate limit for the current key has been exceeded.
type LimitReachedHandler func(c *gin.Context)

// WithLimitReachedHandler will configure the Middleware to use the given LimitReachedHandler.
func WithLimitReachedHandler(handler LimitReachedHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.LimitReachedHandler = handler
	})
}

// DefaultLimitReachedHandler is the default LimitReachedHandler used by a new Middleware.
func DefaultLimitReachedHandler(c *gin.Context) {
	c.String(http.StatusTooManyRequests, "limit exceeded")
}

// KeyGetter will define the rate limiter key given the gin Context.
type KeyGetter func(c *gin.Context) string

// WithKeyGetter will configure the Middleware to use the given KeyGetter.
func WithKeyGetter(handler KeyGetter) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.KeyGetter = handler
	})
}

// DefaultKeyGetter is the default KeyGetter used by a new Middleware.
// It returns the Client IP address.
func DefaultKeyGetter(c *gin.Context) string {
	return c.ClientIP()
}

// WithExcludedKey will configure the Middleware to ignore key(s) using the given function.
func WithExcludedKey(handler func(string) bool) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ExcludedKey = handler
	})
}

// LimiterMiddleware returns Gin middleware that enforces rate limits using limiter, with
// optional configuration via [WithErrorHandler], [WithLimitReachedHandler], [WithKeyGetter],
// and [WithExcludedKey].
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
