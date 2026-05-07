# Repository Guidelines

## Project Structure & Module Organization
`go-clob-client` is a single-module Go library. Source files live at the repository root in package `clobclient`, with responsibilities split by domain: `client.go` for client setup, `auth.go` and `signer.go` for signing/authentication, `account.go` and `orders.go` for private endpoints, `market_data.go` for public market data, and `types.go`/`errors.go` for shared models. Tests sit beside the code in `*_test.go`; live network checks are isolated in `*_integration_test.go`. Supporting research and implementation notes live under `docs/polymarket/`.

## Build, Test, and Development Commands
- `go build ./...` builds the module and catches compile regressions.
- `go test ./... -run '^Test[^I]' -count=1` runs the local deterministic suite while excluding `TestIntegration...` live tests.
- `go test ./... -run '^Test[^I]' -cover` reports statement coverage for the non-live suite; the current baseline is about `73.7%`.
- `go test ./... -run TestIntegrationCreateOrDeriveAPIKey -count=1` runs the live L1 auth test.
- `go test ./... -run TestIntegrationGetBalanceAllowance -count=1` runs the live L2 balance test.
- `go test ./... -run TestIntegrationCreatePostAndCancelOrder -count=1 -v` runs the live GTC post-and-cancel order test.
- `go test ./... -run TestIntegrationCreatePostMarketBuyThenSellOrder -count=1 -v` runs the live market BUY-then-SELL roundtrip test.

## Coding Style & Naming Conventions
Use standard Go formatting: tabs for indentation, `gofmt` before commit, and Go-style exported names (`NewClient`, `BuildL2Headers`) with unexported helpers in lower camel case. Keep files focused on one API area and place tests next to the code they exercise. Name test files `*_test.go`; reserve `*_integration_test.go` for tests that touch real Polymarket services.

## Testing Guidelines
Write table-driven unit tests where practical and keep them deterministic. Integration tests are manually targeted with exact `-run TestIntegration...` commands and read live credentials from `.env`; because live gates have been removed, avoid broad `go test ./...` unless intentionally running production-facing tests. Before opening a PR, run the non-live test and coverage commands above; add or update tests for any public API, auth flow, or error-handling change.

## Commit & Pull Request Guidelines
The existing history follows Conventional Commit style with optional scopes, for example `feat(auth): ...`. Keep commit subjects short and imperative; use a scope when the change is localized. PRs should include a clear summary, the commands you ran (`go test ./...`, coverage, or targeted live tests), and any `.env` or API-behavior changes. For request/response changes, include sample payloads rather than screenshots.

## Security & Configuration Tips
`.env` is ignored and used by live tests. Never commit private keys, API secrets, or real credentials. Treat integration tests as production-facing: they can create or derive live API credentials, place orders, and sell positions, so run them only with an exact `-run TestIntegration...` target and prepared account/token/funder configuration.
