# waybar-next-events

A CLI tool that fetches upcoming Google Calendar events and prints [Waybar](https://github.com/Alexays/Waybar)-compatible JSON.

## Prerequisites

- [Go](https://go.dev/) 1.26+
- [Nerd Font](https://www.nerdfonts.com/)-patched font (e.g., `ttf-jetbrains-mono-nerd`) for icon rendering
- [mise](https://mise.jdx.dev/) (optional, for dev tasks)

## Install

```bash
mise run install
```

Or build manually:

```bash
mise run build
sudo cp bin/waybar-next-events /usr/local/bin/
```

## Quick start

Add a Google Calendar account interactively:

```bash
waybar-next-events account add
```

This launches a form that walks you through OAuth setup. You'll be prompted to create a Google Cloud OAuth client ID (desktop app), enter the client ID and secret, choose calendars, and authenticate in your browser.

> Secrets and OAuth tokens are stored in your OS keyring, not on disk. Accounts are saved to `~/.config/waybar-next-events/config.json`.

## Usage

### List events

```bash
waybar-next-events list
```

Output is Waybar JSON on stdout — safe to use directly in your Waybar config:

```json
{"text":"󰃰 Standup (starts in 15m)","tooltip":"<b>Today</b>\n 9:00AM -  9:30AM    Standup\n\n<b>Tomorrow</b>\n... "}
```

Flags:

- `--days` — look-ahead window in days (default: 4)
- `--limit` — maximum events to show (default: 10)

### Manage accounts

| Command | Description |
|---------|-------------|
| `account add` | Add a new calendar account |
| `account update` | Update an existing account |
| `account delete` | Delete an account |
| `account login` | Re-authenticate an account |

## Waybar integration

Add a custom module to your Waybar config (`~/.config/waybar/config.jsonc`):

```jsonc
"custom/calendar": {
    "exec": "waybar-next-events list --limit 10",
    "return-type": "json",
    "interval": 60,
    "on-click": "waybar-next-events account update"
}
```

Then include `custom/calendar` in your bar modules list.

## Development

Common tasks via mise:

| Task | Command |
|------|---------|
| Run checks | `mise run all` |
| Build | `mise run build` |
| Format | `mise run format` |
| Lint | `mise run lint` |
| Test | `mise run test` |
| Tidy | `mise run tidy` |
| Dev run | `mise run dev` |
| Run a command | `mise run cmd -- list --limit 3` |

Or use `go` directly:

```bash
go run ./cmd/waybar-next-events list --limit 3
```

## Configuration

Account configuration lives at `$HOME/.config/waybar-next-events/config.json`. The directory is created with `0700` permissions and the file with `0600`. Secrets and OAuth tokens are stored in the OS keyring via [`go-keyring`](https://github.com/zalando/go-keyring).
