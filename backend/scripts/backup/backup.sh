#!/usr/bin/env bash
# pg_dump backup script for the Jingdezhen Ceramics Platform (PRD §4.5).
#
# Creates a compressed pg_dump of the production database and uploads it to
# Alibaba Cloud OSS (or S3-compatible storage). Designed to run as a nightly
# cron job on the VPS.
#
# RPO: 24h (nightly). RTO: 4h (restore via pg_restore, see restore.sh).
#
# Usage (cron):
#   0 2 * * * /opt/jdz/scripts/backup.sh >> /var/log/jdz-backup.log 2>&1
#
# Required env (set in /opt/jdz/.env or the systemd env file):
#   DATABASE_URL=postgres://user:pass@localhost:5432/jingdezhen
#   OSS_ACCESS_KEY_ID=...
#   OSS_ACCESS_KEY_SECRET=...
#   OSS_BUCKET=jingdezhen-backups
#   OSS_ENDPOINT=oss-cn-hongkong.aliyuncs.com
#   # Optional: OSS_PREFIX=jingdezhen/pg/ (default)
#   # Optional: RETENTION_DAYS=14 (default)
set -euo pipefail

# --- Config ---
DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required}"
OSS_BUCKET="${OSS_BUCKET:?OSS_BUCKET is required}"
OSS_ENDPOINT="${OSS_ENDPOINT:?OSS_ENDPOINT is required}"
OSS_ACCESS_KEY_ID="${OSS_ACCESS_KEY_ID:?OSS_ACCESS_KEY_ID is required}"
OSS_ACCESS_KEY_SECRET="${OSS_ACCESS_KEY_SECRET:?OSS_ACCESS_KEY_SECRET is required}"
OSS_PREFIX="${OSS_PREFIX:-jingdezhen/pg/}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

# --- Timestamp ---
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_FILE="jingdezhen-${TIMESTAMP}.dump.gz"
BACKUP_PATH="/tmp/${BACKUP_FILE}"

echo "[$(date -u)] Starting backup → ${BACKUP_FILE}"

# --- pg_dump (custom format + compressed) ---
# Using --format=custom for parallel restore (pg_restore -j).
# --no-owner + --no-privileges so restore can target a fresh DB without
# role/ownership conflicts.
PGPASSWORD=$(echo "${DATABASE_URL}" | sed -n 's/.*:\([^@]*\)@.*/\1/p') \
  pg_dump \
    --format=custom \
    --no-owner \
    --no-privileges \
    --compress=6 \
    --file="${BACKUP_PATH}" \
    "${DATABASE_URL}"

BACKUP_SIZE=$(stat -c%s "${BACKUP_PATH}" 2>/dev/null || stat -f%z "${BACKUP_PATH}")
echo "[$(date -u)] pg_dump complete: ${BACKUP_FILE} (${BACKUP_SIZE} bytes)"

# --- Upload to OSS ---
# Uses ossutil (Alibaba Cloud OSS CLI). Install:
#   https://www.alibabacloud.com/help/en/oss/developer-reference/ossutil
# OSS is S3-compatible; you can substitute aws s3 cp if preferred.
OSS_KEY="${OSS_PREFIX}${BACKUP_FILE}"

ossutil cp "${BACKUP_PATH}" "oss://${OSS_BUCKET}/${OSS_KEY}" \
  -i "${OSS_ACCESS_KEY_ID}" \
  -k "${OSS_ACCESS_KEY_SECRET}" \
  -e "${OSS_ENDPOINT}" \
  --force

echo "[$(date -u)] Uploaded to oss://${OSS_BUCKET}/${OSS_KEY}"

# --- Retention: prune backups older than RETENTION_DAYS ---
PRUNE_DATE=$(date -u -d "${RETENTION_DAYS} days ago" +"%Y%m%d" 2>/dev/null || \
  date -u -v-${RETENTION_DAYS}d +"%Y%m%d" 2>/dev/null || \
  echo "")
if [ -n "${PRUNE_DATE}" ]; then
  echo "[$(date -u)] Pruning backups older than ${PRUNE_DATE}"
  ossutil ls "oss://${OSS_BUCKET}/${OSS_PREFIX}" \
    -i "${OSS_ACCESS_KEY_ID}" \
    -k "${OSS_ACCESS_KEY_SECRET}" \
    -e "${OSS_ENDPOINT}" \
    --limited-num 100 | \
    awk -v cutoff="${PRUNE_DATE}" '/jingdezhen-[0-9]/ {print $NF}' | \
    while read -r old_key; do
      file_date=$(echo "${old_key}" | sed -n 's/.*jingdezhen-\([0-9T]*\).*/\1/p' | cut -dT -f1)
      if [ -n "${file_date}" ] && [ "${file_date}" \< "${cutoff}" ]; then
        ossutil rm "oss://${OSS_BUCKET}/${old_key}" \
          -i "${OSS_ACCESS_KEY_ID}" \
          -k "${OSS_ACCESS_KEY_SECRET}" \
          -e "${OSS_ENDPOINT}" \
          --force || true
        echo "[$(date -u)] Pruned: ${old_key}"
      fi
    done
fi

# --- Cleanup local temp file ---
rm -f "${BACKUP_PATH}"
echo "[$(date -u)] Backup complete: ${BACKUP_FILE} (${BACKUP_SIZE} bytes → oss://${OSS_BUCKET}/${OSS_KEY})"
