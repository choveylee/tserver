// Package tmiddleware provides Gin middleware for cross-cutting HTTP concerns, including
// CORS headers, rate limiting, and request body buffering so downstream handlers can
// safely read the body more than once when required.
package tmiddleware
