# backuperr

Simple Go backup system with:
- `host`: HTTP server storing backups per client IP
- `client`: backup/restore CLI + interactive TUI menu

Supports full + incremental backups, restore with prune (deleted files are removed on restore), optional cron scheduling (Linux), and optional webhooks (including Discord).

## Features

- Full and incremental backups
- Backup metadata + manifest storage on host
- Restore from backup chain (full + incrementals)
- Prune files not present in final manifest after restore
- Interactive client menu (`./client` with no args)
- Human-friendly backup time display (`2 days ago`)
- Linux cron schedule setup from client menu
- Optional webhook notifications from host and client
- Discord webhook auto-detection with rich embed output

## Install
client:
```bash
curl -sSL https://raw.githubusercontent.com/hdmain/backuperr/main/install_client.sh | bash
```
host:
```bash
curl -sSL https://raw.githubusercontent.com/hdmain/backuperr/main/install_host.sh | bash
```
## Project layout

- `cmd/host`: host entrypoint
- `cmd/client`: client entrypoint
- `internal/host`: host API, storage, disk usage
- `internal/client`: backup/restore logic, TUI, schedule, webhooks
- `pkg/types`: shared API/data types

## Requirements

- Go `1.22+`(if build)
- Linux recommended for production use (cron + disk reporting)

## Build

```bash
go build -o host ./cmd/host
go build -o client ./cmd/client
```

## Configuration

Use provided examples:
- `config.yml.example` -> `config.yml` (host)
- `client.yaml.example` -> `client.yaml` (client)

### Host config (`config.yml`)

```yaml
listen: ":8443"
data_dir: "./data"
main_key: "change-this-main-key"
# webhook_url: "https://discord.com/api/webhooks/..."
```

### Client config (`client.yaml`)

```yaml
url: http://127.0.0.1:8443
api_key: change-this-main-key
backup_root: /home/you/data
# restore_to: /home/you/restored
# state_path: /home/you/.backuperr-state.json
# temp_dir: /var/tmp
# webhook_url: "https://discord.com/api/webhooks/..."
exclude:
  - ".git"
  - "node_modules"
```

## Run

Start host:

```bash
./host -config config.yml
```

Run client interactive menu:

```bash
./client -config client.yaml
```

Or run commands directly:

```bash
./client -config client.yaml backup
./client -config client.yaml backup --full
./client -config client.yaml list
./client -config client.yaml restore
```

## Schedule backups (Linux cron)

From client menu:
1. Run `./client -config client.yaml`
2. Open `Schedule backups (cron)`
3. Pick preset or enter custom 5-field cron expression

This installs/updates a managed block in your **user crontab** and runs:

```bash
client -config /absolute/path/client.yaml backup
```

You can also remove schedule from the same menu.

## Webhooks

Set `webhook_url` in host/client config to receive status notifications.

- Host events: `startup`, `backup_received`, `backup_failed`
- Client events: `backup_complete`, `backup_failed`, `restore_complete`, `restore_failed`

Payload includes status, timestamp, and free/total disk fields.

### Discord webhooks

If URL matches Discord incoming webhook format (`discord.com/api/webhooks/...`), backuperr sends a Discord embed with readable fields and status color.

For non-Discord URLs, it sends plain JSON.

## Notes

- Host authorizes requests via `X-API-Key` (`main_key` / `api_key`)
- Client identity on host is based on client IP (`X-Forwarded-For` respected)
- On non-Linux systems, cron setup is not supported
- For large backups, configure `temp_dir` if `/tmp` is small
