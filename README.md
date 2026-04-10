# nsxt-fw-backup

CLI utility to **back up** and **restore** VMware NSX-T **distributed firewall** configuration via the **Policy API** (`/policy/api/v1`). Backups are a single JSON document containing full API payloads keyed by canonical `/infra/...` paths.

Test against your NSX-T version; rule and policy shapes can vary slightly between releases.

## Requirements

- NSX-T Manager reachable over HTTPS
- Policy API credentials (see below)

## Install / build

From the repository root, use the [Makefile](Makefile) (pure Go, `CGO_ENABLED=0`):

```bash
# Current OS/arch → dist/nsxt-fw-backup
make build

# Linux, Windows, and Darwin for amd64 and arm64 → dist/
make build-all

# Remove build output
make clean

# Target summary
make help
```

Output directory defaults to `dist/`; override with `make build DIST=out` or `make build-all DIST=out`.

After `make build`, invoke `./dist/nsxt-fw-backup` in place of `./nsxt-fw-backup` in the examples below.

To build without Make:

```bash
go build -o nsxt-fw-backup ./cmd/nsxt-fw-backup
```

## Environment variables (authentication)

Credentials are **never** accepted on the command line.

| Variable | Purpose |
|----------|---------|
| `NSXT_MANAGER_HOST` or `NSXT_HOST` | Manager hostname or `https://host[:port]` (overridden by `--host` when set) |
| `NSXT_USERNAME` | Basic auth user |
| `NSXT_PASSWORD` | Basic auth password |
| `NSXT_BEARER_TOKEN` or `NSXT_API_KEY` | Bearer token (do not set username/password if using this) |
| `NSXT_INSECURE_SKIP_TLS_VERIFY` | Set to `true` / `1` / `yes` to skip TLS verification (lab only) |

## Commands

### Backup

Exports all security policies in the domain (or one policy by display name), their rules, and referenced objects (groups, services, context profiles) reachable from those rules.

```bash
export NSXT_MANAGER_HOST=https://nsx.example.com
export NSXT_USERNAME=admin
export NSXT_PASSWORD='secret'

./nsxt-fw-backup backup --output backup.json

# Single section (security policy display_name exact match)
./nsxt-fw-backup backup --output section.json --section "App firewall"

# Multi-tenant Policy path prefix (both org and project required)
./nsxt-fw-backup backup --org default --project tenant-a -o tenant.json

# Omit manager hostname from the JSON file
./nsxt-fw-backup backup -o backup.json --redact-host
```

Global flags: `--host`, `--domain` (default `default`), `--org`, `--project`, `--insecure-skip-tls-verify`.

### Restore

1. By default, loads the backup, **GET**s each resource on the manager, and prints a **dry-run** table (`CREATE` / `SKIP` / `UPDATE`).
2. Prompts **Proceed? [y/N]** unless `-y` / `--yes`.
3. **SKIP** when the object exists unless `--force` (then **UPDATE** via PUT).
4. `--skip-dry-run` hides the table but **must** be used with **`-y`** (safety guard).

```bash
./nsxt-fw-backup restore --input backup.json
./nsxt-fw-backup restore -i backup.json --yes
./nsxt-fw-backup restore -i backup.json --force
./nsxt-fw-backup restore -i backup.json --skip-dry-run -y
```

If you do not pass `--org` / `--project`, the tool uses `api_prefix` from the backup file’s `scope` when present.

## Backup file format

- `format_version`: currently `1`
- `scope`: `domain`, optional `section`, `org`, `project`, `api_prefix`
- `resources`: map of `/infra/...` path → raw JSON object from the Policy API

Security policies are stored **without** an inline `rules` array; rules are separate entries under `.../security-policies/{id}/rules/{rule-id}`.

## Limitations

- **Distributed firewall only** (not gateway firewall).
- Restore uses **PUT** with bodies from the backup; some environments may require ETag/`If-Match` for updates—if PUT fails, check manager logs and API docs for your version.
- Very large policies may require tuning HTTP timeouts in code for your environment.

## License

This project is licensed under the [MIT License](LICENSE).
