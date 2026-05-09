# waybar-next-events

A CLI tool that fetches upcoming Google Calendar events and prints [Waybar](https://github.com/Alexays/Waybar)-compatible JSON.

## Prerequisites

- [Nerd Font](https://www.nerdfonts.com/) (recommended, for icon rendering)

## Install

Install from a GitHub release:

1. Open the [latest releases page](https://github.com/hossainemruz/waybar-next-events/releases/latest)
2. Download the archive for your system:
   - `waybar-next-events_<version>_linux_amd64.tar.gz`
   - `waybar-next-events_<version>_linux_arm64.tar.gz`
3. Extract and install the binary:

```bash
tar -xzf waybar-next-events_<version>_linux_amd64.tar.gz
sudo install -m 0755 waybar-next-events /usr/local/bin/waybar-next-events
```

Or install a specific release directly with `curl` (replace `VERSION` with the desired tag):

```bash
VERSION=v0.1.0
ARCH=amd64 # or arm64
curl -fsSL -o /tmp/waybar-next-events.tar.gz \
  "https://github.com/hossainemruz/waybar-next-events/releases/download/${VERSION}/waybar-next-events_${VERSION#v}_linux_${ARCH}.tar.gz"
tar -xzf /tmp/waybar-next-events.tar.gz -C /tmp
sudo install -m 0755 /tmp/waybar-next-events /usr/local/bin/waybar-next-events
```

For local development installs:

```bash
mise run install
```

Or build manually without `mise`:

```bash
go build -o /tmp/waybar-next-events ./cmd/waybar-next-events
sudo install -m 0755 /tmp/waybar-next-events /usr/local/bin/waybar-next-events
```

## Updating

To update to a newer release, re-run the install steps above with the new version. The binary will be overwritten in `/usr/local/bin/`.

## Quick start

Add a Google Calendar account interactively:

```bash
waybar-next-events account add
```

This launches a form that walks you through OAuth setup. You'll be prompted to create a Google Cloud OAuth client ID (desktop app), enter the client ID and secret, choose calendars, and authenticate in your browser.

> Secrets and OAuth tokens are stored in your OS keyring, not on disk. Accounts are saved to `~/.config/waybar-next-events/config.json`.
>
> You can add multiple accounts by running `account add` again. Events from all accounts are merged and sorted by start time.

### Create a Google OAuth client

To add an account, first create a Google OAuth client ID and secret:

1. Open the [Google Cloud Console](https://console.cloud.google.com/)
2. Create or select a project
3. Enable the **Google Calendar API** for that project
4. Go to **APIs & Services** → **Credentials**
5. Click **Create Credentials** → **OAuth client ID**
6. If prompted, configure the OAuth consent screen first:
   - choose **External** for personal use
   - fill in the required app information
   - add yourself as a test user if Google asks for it
7. For application type, choose **Desktop app**
8. Create the client and copy the **Client ID** and **Client secret**

Then run:

```bash
waybar-next-events account add
```

When prompted, paste the client ID and client secret. During login, the app uses this fixed callback URL:

```text
http://127.0.0.1:18751/callback
```

If Google shows an "app isn't verified" warning for your own client, continue with the advanced prompt and authorize it for your account.

## Usage

### List events

```bash
waybar-next-events list
```

Output is Waybar JSON on stdout — safe to use directly in your Waybar config:

```json
{"text":"󰃰 Standup (starts in 15m)","tooltip":"<b>Today</b>\n 9:00AM -  9:30AM    Standup\n\n<b>Tomorrow</b>\n... "}
```

When there are no more events today:

```json
{"text":" No more events today!","tooltip":"<b>Today</b>\nAll day              Lunch with team\n... "}
```

Flags:

- `--days` — look-ahead window in days (default: 4)
- `--limit` — maximum events to show (default: 10)

Run `waybar-next-events list --help` to see all available flags.

> **Multi-account behavior:** `list` is fail-fast. If any configured account fails to fetch events, the entire command aborts and no output is produced.

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
    "exec": "waybar-next-events list --days 4 --limit 10",
    "return-type": "json",
    "interval": 60
}
```

Then include `custom/calendar` in your bar modules list.

## Development

You will need [Go](https://go.dev/) 1.26+ and [mise](https://mise.jdx.dev/) to run the development tasks below.

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
| Run a command | `mise run cmd list -- --limit 3` |

Or use `go` directly:

```bash
go run ./cmd/waybar-next-events list --limit 3
```

## Configuration

Account configuration lives at `$HOME/.config/waybar-next-events/config.json`. The directory is created with `0700` permissions and the file with `0600`. Secrets and OAuth tokens are stored in the OS keyring via [`go-keyring`](https://github.com/zalando/go-keyring).
