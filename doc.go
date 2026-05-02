// Package tserver provides HTTP server construction and lifecycle helpers for applications
// built with the Gin web framework. It integrates OpenTelemetry tracing,
// Prometheus-compatible request latency metrics, structured access logging, and graceful
// shutdown in response to context cancellation or operating-system signals (SIGINT and
// SIGTERM).
package tserver
