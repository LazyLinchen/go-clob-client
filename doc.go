// Package clobclient provides a Go client for the Polymarket CLOB API.
//
// This first iteration focuses on:
//   - a reusable, concurrency-safe HTTP client core
//   - public market-data endpoints that do not require authentication
//
// A Client is intended to be long-lived and reused across requests. By default,
// clients share a pooled HTTP transport so connections can be kept alive and
// reused instead of creating a new transport per request.
package clobclient
