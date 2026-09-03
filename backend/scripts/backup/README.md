# Backup & Restore (PRD §4.5)

Nightly `pg_dump` → Alibaba Cloud OSS. RPO 24h, RTO 4h.

## Scripts

| Script | Purpose |
|--------|---------|
| `backup.sh` | Compressed pg_dump (custom format) → OSS upload + retention prune |
| `restore.sh` | Download latest (or specific) backup from OSS → pg_restore |

## Setup

### 1. Install ossutil (Alibaba Cloud OSS CLI)

```bash
# Download the latest ossutil binary
curl -o /usr/local/bin/ossutil https://gosspublic.alicdn.com/ossutil/1.7.18/ossutil64
chmod +x /usr/local/bin/ossutil
```

### 2. Environment variables

Set in `/opt/jdz/.env` (or the systemd/cron env file):

```bash
DATABASE_URL=postgres://jdz:password@localhost:5432/jingdezhen
OSS_ACCESS_KEY_ID=LTAI...
OSS_ACCESS_KEY_SECRET=...
OSS_BUCKET=jingdezhen-backups
OSS_ENDPOINT=oss-cn-hongkong.aliyuncs.com
OSS_PREFIX=jingdezhen/pg/     # optional, default shown
RETENTION_DAYS=14             # optional, default shown
```

### 3. Cron schedule (nightly at 2:00 AM)

```cron
0 2 * * * . /opt/jdz/.env && /opt/jdz/scripts/backup/backup.sh >> /var/log/jdz-backup.log 2>&1
```

## Restore (disaster recovery)

### From the latest backup

```bash
. /opt/jdz/.env
./scripts/backup/restore.sh
```

### From a specific backup

```bash
. /opt/jdz/.env
./scripts/backup/restore.sh jingdezhen-20260901T020000Z.dump.gz
```

### Backup/restore drill (PRD M4 exit criteria)

1. Run `backup.sh` on the production VPS.
2. Verify the backup appears in OSS (`ossutil ls oss://jingdezhen-backups/jingdezhen/pg/`).
3. Spin up a fresh VPS (same spec) with Docker Compose.
4. Run `restore.sh` targeting the fresh DB.
5. Verify: `make migrate-up` (no-op — schema is in the dump), then `curl localhost:1323/catalog/products?limit=1` returns 200.
6. Record the total restore time — must be < 4h (RTO).

## Design notes

- `pg_dump --format=custom --compress=6` — custom format enables
  parallel `pg_restore -j 4` for faster restore (RTO target).
- `--no-owner --no-privileges` — restore works on a fresh DB without
  role/ownership conflicts.
- Retention: 14 days default (configurable via `RETENTION_DAYS`). Pruned
  after each backup run.
- OSS endpoint: Hong Kong region (non-mainland, ICP-free, matches PRD §2.1).
- Weekly full VPS snapshot (Docker volumes + config) is a separate
  hypervisor-level task — not covered by these scripts.
