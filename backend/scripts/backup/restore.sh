#!/usr/bin/env bash
# Restore script for the Jingdezhen Ceramics Platform (PRD §4.5).
#
# Downloads the latest (or a specific) backup from OSS and restores it to
# a target database via pg_restore. RTO target: 4h (PRD §4.5 line 363).
#
# Usage:
#   ./restore.sh                          # restore latest backup to DATABASE_URL
#   ./restore.sh jingdezhen-20260901T020000Z.dump.gz  # restore a specific backup
#
# Required env:
#   DATABASE_URL=postgres://user:pass@localhost:5432/jingdezhen
#   OSS_ACCESS_KEY_ID=...
#   OSS_ACCESS_KEY_SECRET=...
#   OSS_BUCKET=jingdezhen-backups
#   OSS_ENDPOINT=oss-cn-hongkong.aliyuncs.com
#   # Optional: OSS_PREFIX=jingdezhen/pg/ (must match backup.sh)
set -euo pipefail

DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required}"
OSS_BUCKET="${OSS_BUCKET:?OSS_BUCKET is required}"
OSS_ENDPOINT="${OSS_ENDPOINT:?OSS_ENDPOINT is required}"
OSS_ACCESS_KEY_ID="${OSS_ACCESS_KEY_ID:?OSS_ACCESS_KEY_ID is required}"
OSS_ACCESS_KEY_SECRET="${OSS_ACCESS_KEY_SECRET:?OSS_ACCESS_KEY_SECRET is required}"
OSS_PREFIX="${OSS_PREFIX:-jingdezhen/pg/}"

# --- Determine which backup to restore ---
SPECIFIC="${1:-}"
if [ -n "${SPECIFIC}" ]; then
  BACKUP_FILE="${SPECIFIC}"
else
  echo "[$(date -u)] Listing backups in oss://${OSS_BUCKET}/${OSS_PREFIX}..."
  # Find the most recent backup (sorted by filename = sorted by timestamp).
  BACKUP_FILE=$(ossutil ls "oss://${OSS_BUCKET}/${OSS_PREFIX}" \
    -i "${OSS_ACCESS_KEY_ID}" \
    -k "${OSS_ACCESS_KEY_SECRET}" \
    -e "${OSS_ENDPOINT}" \
    --limited-num 100 | \
    grep -o 'jingdezhen-[0-9T]*\.dump\.gz' | \
    sort -r | \
    head -1)
  if [ -z "${BACKUP_FILE}" ]; then
    echo "ERROR: No backups found in oss://${OSS_BUCKET}/${OSS_PREFIX}"
    exit 1
  fi
fi

OSS_KEY="${OSS_PREFIX}${BACKUP_FILE}"
DOWNLOAD_PATH="/tmp/${BACKUP_FILE}"

echo "[$(date -u)] Restoring from oss://${OSS_BUCKET}/${OSS_KEY}"

# --- Download from OSS ---
ossutil cp "oss://${OSS_BUCKET}/${OSS_KEY}" "${DOWNLOAD_PATH}" \
  -i "${OSS_ACCESS_KEY_ID}" \
  -k "${OSS_ACCESS_KEY_SECRET}" \
  -e "${OSS_ENDPOINT}" \
  --force

DOWNLOAD_SIZE=$(stat -c%s "${DOWNLOAD_PATH}" 2>/dev/null || stat -f%z "${DOWNLOAD_PATH}")
echo "[$(date -u)] Downloaded: ${BACKUP_FILE} (${DOWNLOAD_SIZE} bytes)"

# --- Decompress ---
DECOMPRESSED_PATH="/tmp/jingdezhen-restore-${$}.dump"
gunzip -c "${DOWNLOAD_PATH}" > "${DECOMPRESSED_PATH}"
echo "[$(date -u)] Decompressed: ${DECOMPRESSED_PATH}"

# --- pg_restore ---
# pg_restore with --clean --if-exists drops existing objects before recreating
# (idempotent restore). --no-owner + --no-privileges avoids role conflicts.
# -j 4 uses 4 parallel jobs for speed (RTO: 4h target).
echo "[$(date -u)] Starting pg_restore → ${DATABASE_URL}"
PGPASSWORD=$(echo "${DATABASE_URL}" | sed -n 's/.*:\([^@]*\)@.*/\1/p') \
  pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --jobs=4 \
    --dbname="${DATABASE_URL}" \
    "${DECOMPRESSED_PATH}"

echo "[$(date -u)] Restore complete: ${BACKUP_FILE}"

# --- Cleanup ---
rm -f "${DOWNLOAD_PATH}" "${DECOMPRESSED_PATH}"
echo "[$(date -u)] Temp files cleaned up"
