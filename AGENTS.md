# AGENTS.md — waybar-next-events

Go CLI that fetches Google Calendar events and prints Waybar JSON.

## Fast path

- Run `mise run all` (build, format, lint, test, tidy) before marking any task complete.
- Prefer `gopls` for definitions, references, implementations, and callers over manual text search.
- Use `go doc ./internal/<package>` when exported package or API behavior/signatures need clarification.

## Common tasks

- Dev run: `mise run dev` or `go run ./cmd/waybar-next-events`
- Run CLI commands: `mise run cmd -- <subcommand> <args>`

## Wiring

- Entrypoint: `cmd/waybar-next-events/main.go`
- Startup: `app.NewRegistry()` → register `internal/services/google.NewService()` → `cli.Execute(registry)`
- Top-level Cobra commands only:
  - `list` — prints Waybar JSON
  - `account` — interactive `add`, `update`, `delete`, `login`

## Where to add code

- New top-level commands go in `internal/cli/commands/` and are wired in `root.go`.
- Account subcommands live in `internal/cli/commands/account_*.go`.

## Repo-specific behavior

- `list` goes through `internal/app/EventFetcher`: load config, resolve each account service, build provider + OAuth client, fetch events, sort by start time, apply limit, then render.
- Multi-account fetch is fail-fast: one account error aborts the whole command.
- `internal/output.Render` expects events already sorted. Do **not** move sorting out of `EventFetcher`.
- Config lives at `$HOME/.config/waybar-next-events/config.json`; saves normalized account/calendar ordering and use `0700` dir + `0600` file perms.
- OAuth callback URL is fixed to `http://127.0.0.1:18751/callback`; provider validation must stay aligned.
- Secrets and OAuth tokens live in the OS keyring, not the config file.
- `.env` is gitignored but not loaded by the app.
- `list` is Waybar-facing: keep JSON on stdout. Write interactive prompts and errors to stderr so Waybar parsing isn't broken.
- Interactive account flows use `charm.huh/v2` forms.

## Tests

- Tests are mostly focused single-case unit tests with fakes/stubs, not table-driven suites.
- Use fixed `time.Date(...)` values in tests; avoid `time.Now()`.
- Use `secrets.NewMemStore()` and `tokenstore.NewMemStore()` in tests to avoid OS keyring dependencies.
- Command test helpers live in `internal/cli/commands/test_helpers.go`.
- For `list` output assertions, unmarshal into `output.WaybarPayload` instead of comparing raw JSON.
