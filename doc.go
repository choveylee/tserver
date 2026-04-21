// Package tserver provides HTTP server construction and lifecycle helpers for applications
// built on the Gin web framework. It integrates OpenTelemetry tracing, Prometheus-style
// request latency metrics, structured access logging, and graceful shutdown in response to
// context cancellation or OS signals (SIGINT, SIGTERM).
package tserver
