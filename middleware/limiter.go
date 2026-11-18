package tmiddleware

import (
	"errors"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/choveylee/tlimiter"
	"github.com/choveylee/tlog"
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
// request is allowed. Excluded keys bypass quota consumption and rate-limit headers.
// Allowed requests expose the updated quota, including zero remaining on the last
// admitted event.
func (config *LimiterConfig) Handle(ginContext *gin.Context) {
	errorHandler := config.ErrorHandler
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
	}
	if config.Limiter == nil || config.Limiter.Store == nil {
		err := errors.New("tserver: rate limiter and store must be configured")
		errorHandler(ginContext, err)
		ginContext.Abort()

		return
	}

	var key string
	if config.KeyGetter != nil {
		key = config.KeyGetter(ginContext)
	} else {
		key = maskedClientIPKey(ginContext, config.Limiter)
	}

	if config.ExcludedKey != nil && config.ExcludedKey(key) {
		ginContext.Next()

		return
	}

	// Pass the HTTP request context so its cancellation and deadline reach the store.
	state, err := config.Limiter.Get(ginContext.Request.Context(), key)
	if err != nil {
		errorHandler(ginContext, err)

		ginContext.Abort()

		return
	}

	ginContext.Header("X-RateLimit-Limit", strconv.FormatInt(state.Limit, 10))
	ginContext.Header("X-RateLimit-Remaining", strconv.FormatInt(state.Remaining, 10))
	ginContext.Header("X-RateLimit-Reset", strconv.FormatInt(state.Reset, 10))

	if state.Reached {
		handler := config.LimitReachedHandler
		if handler == nil {
			handler = DefaultLimitReachedHandler
		}
		handler(ginContext)

		ginContext.Abort()

		return
	}

	ginContext.Next()
}

// LimiterConfigOptionInterface is implemented by functional options for
// [LimiterMiddleware].
type LimiterConfigOptionInterface interface {
	apply(*LimiterConfig)
}

// LimiterConfigOption is a functional option that mutates a [LimiterConfig] when applied.
type LimiterConfigOption func(*LimiterConfig)

func (option LimiterConfigOption) apply(config *LimiterConfig) {
	if option != nil {
		option(config)
	}
}

// ErrorHandler handles errors returned by the limiter while resolving state for the
// current request key.
type ErrorHandler func(ginContext *gin.Context, err error)

// WithErrorHandler configures [LimiterMiddleware] to use the provided [ErrorHandler].
func WithErrorHandler(handler ErrorHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ErrorHandler = handler
	})
}

// DefaultErrorHandler rejects the request on a store or configuration failure.
func DefaultErrorHandler(ginContext *gin.Context, err error) {
	tlog.E(ginContext.Request.Context()).
		Detailf("error_type:%T", err).
		Msg("Apply HTTP rate limit")
	ginContext.AbortWithStatus(http.StatusInternalServerError)
}

// LimitReachedHandler is invoked when the rate limit for the current key has been exceeded.
type LimitReachedHandler func(ginContext *gin.Context)

// WithLimitReachedHandler configures [LimiterMiddleware] to use the provided
// [LimitReachedHandler].
func WithLimitReachedHandler(handler LimitReachedHandler) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.LimitReachedHandler = handler
	})
}

// DefaultLimitReachedHandler is the default [LimitReachedHandler] used by
// [LimiterMiddleware].
func DefaultLimitReachedHandler(ginContext *gin.Context) {
	ginContext.String(http.StatusTooManyRequests, "limit exceeded")
}

// KeyGetter derives the rate-limit key from the current Gin context.
type KeyGetter func(ginContext *gin.Context) string

// WithKeyGetter configures [LimiterMiddleware] to use the provided [KeyGetter].
func WithKeyGetter(handler KeyGetter) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.KeyGetter = handler
	})
}

// DefaultKeyGetter returns the client IP resolved by Gin's proxy settings.
// LimiterMiddleware applies the limiter's IP mask to this value unless a custom
// KeyGetter is supplied.
func DefaultKeyGetter(ginContext *gin.Context) string {
	return ginContext.ClientIP()
}

// maskedClientIPKey applies the limiter's subnet to Gin's trusted client IP.
func maskedClientIPKey(ginContext *gin.Context, limiter *tlimiter.Limiter) string {
	key := DefaultKeyGetter(ginContext)
	clientIP := net.ParseIP(key)
	mask := limiter.Options.IPv6Mask
	if clientIP.To4() != nil {
		mask = limiter.Options.IPv4Mask
	}

	maskedIP := clientIP.Mask(mask)
	if maskedIP == nil {
		return key
	}

	return maskedIP.String()
}

// WithExcludedKey configures [LimiterMiddleware] to skip rate limiting for keys for which
// the provided predicate returns true.
func WithExcludedKey(handler func(string) bool) LimiterConfigOptionInterface {
	return LimiterConfigOption(func(config *LimiterConfig) {
		config.ExcludedKey = handler
	})
}

// LimiterMiddleware returns Gin middleware that enforces rate limits using limiter.
// The default key applies limiter.Options.IPv4Mask or IPv6Mask to Gin's client IP.
// A custom KeyGetter replaces this behavior and its result is used without masking.
// Additional behavior may be customized with [WithErrorHandler],
// [WithLimitReachedHandler], [WithKeyGetter], and [WithExcludedKey].
func LimiterMiddleware(limiter *tlimiter.Limiter, options ...LimiterConfigOptionInterface) gin.HandlerFunc {
	limiterConfig := &LimiterConfig{Limiter: limiter}
	for _, option := range options {
		if option != nil {
			option.apply(limiterConfig)
		}
	}

	return limiterConfig.Handle
}
