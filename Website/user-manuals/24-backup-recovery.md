# User Manual 24: Backup and Recovery

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Backup Architecture](#backup-architecture)
4. [PostgreSQL Backup](#postgresql-backup)
5. [Redis Persistence and Backup](#redis-persistence-and-backup)
6. [Configuration Backup](#configuration-backup)
7. [Vector Database Backup](#vector-database-backup)
8. [Automated Backup Schedule](#automated-backup-schedule)
9. [Recovery Procedures](#recovery-procedures)
10. [Point-in-Time Recovery](#point-in-time-recovery)
11. [Backup Verification](#backup-verification)
12. [Retention Policies](#retention-policies)
13. [Backup Storage](#backup-storage)
14. [Troubleshooting](#troubleshooting)
15. [Related Resources](#related-resources)

## Overview

HelixAgent stores persistent data across multiple systems: PostgreSQL (debate sessions, debate turns, code versions, user data), Redis (cache, rate limiter state, session data), vector databases (Qdrant, Pinecone, Milvus, pgvector embeddings), and configuration files. This manual covers backup procedures for each system, automated scheduling, recovery processes, and verification steps.

## Prerequisites

- `pg_dump` and `psql` CLI tools (included with PostgreSQL client)
- `redis-cli` for Redis backup operations
- Access to the backup storage location (local disk, S3, or remote host)
- SSH access to remote hosts if using distributed containers
- Sufficient disk space for backup retention (estimate: 2x current data size)

## Backup Architecture

```
+------------------+     +------------------+     +------------------+
|   PostgreSQL     |     |     Redis        |     |  Vector DBs      |
|  (debate data,   |     |  (cache, rate    |     |  (Qdrant,        |
|   sessions)      |     |   limits)        |     |   embeddings)    |
+--------+---------+     +--------+---------+     +--------+---------+
         |                        |                        |
         v                        v                        v
+--------+---------+     +--------+---------+     +--------+---------+
| pg_dump / WAL    |     | RDB + AOF        |     | Volume snapshots |
| archiving        |     | snapshots        |     | / API export     |
+--------+---------+     +--------+---------+     +--------+---------+
         |                        |                        |
         +------------------------+------------------------+
                                  |
                         +--------v---------+
                         |  Backup Storage  |
                         |  (local / S3 /   |
                         |   remote host)   |
                         +------------------+
```

## PostgreSQL Backup

### Full Logical Backup

```bash
# Full database dump (compressed)
pg_dump -h localhost -p 15432 -U helixagent -d helixagent_db \
    --format=custom --compress=9 \
    -f /backups/postgres/helixagent_db_$(date +%Y%m%d_%H%M%S).dump

# Full database dump (plain SQL, for portability)
pg_dump -h localhost -p 15432 -U helixagent -d helixagent_db \
    > /backups/postgres/helixagent_db_$(date +%Y%m%d_%H%M%S).sql
```

### Schema-Only Backup

```bash
# Backup only the schema (useful for migrations)
pg_dump -h localhost -p 15432 -U helixagent -d helixagent_db \
    --schema-only \
    -f /backups/postgres/schema_$(date +%Y%m%d_%H%M%S).sql
```

### Table-Specific Backup

```bash
# Backup specific tables
pg_dump -h localhost -p 15432 -U helixagent -d helixagent_db \
    --table=debate_sessions \
    --table=debate_turns \
    --table=code_versions \
    -f /backups/postgres/debate_data_$(date +%Y%m%d_%H%M%S).sql
```

### WAL Archiving for Continuous Backup

Enable Write-Ahead Log archiving in `postgresql.conf`:

```
wal_level = replica
archive_mode = on
archive_command = 'cp %p /backups/postgres/wal/%f'
archive_timeout = 300
```

This enables point-in-time recovery to any moment within the WAL retention period.

### Automated Backup Script

```bash
#!/bin/bash
# scripts/backup-database.sh
set -euo pipefail

BACKUP_DIR="/backups/postgres"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-15432}"
DB_USER="${DB_USER:-helixagent}"
DB_NAME="${DB_NAME:-helixagent_db}"
RETENTION_DAYS=30

mkdir -p "${BACKUP_DIR}"

echo "[$(date)] Starting PostgreSQL backup..."

PGPASSWORD="${DB_PASSWORD}" pg_dump \
    -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
    --format=custom --compress=9 \
    -f "${BACKUP_DIR}/${DB_NAME}_${TIMESTAMP}.dump"

BACKUP_SIZE=$(du -h "${BACKUP_DIR}/${DB_NAME}_${TIMESTAMP}.dump" | cut -f1)
echo "[$(date)] Backup complete: ${BACKUP_SIZE}"

# Clean up old backups
find "${BACKUP_DIR}" -name "*.dump" -mtime +${RETENTION_DAYS} -delete
echo "[$(date)] Cleaned backups older than ${RETENTION_DAYS} days"
```

## Redis Persistence and Backup

### RDB Snapshots

Redis RDB persistence creates point-in-time snapshots. Configure in `redis.conf`:

```
save 900 1      # Save after 900 seconds if at least 1 key changed
save 300 10     # Save after 300 seconds if at least 10 keys changed
save 60 10000   # Save after 60 seconds if at least 10000 keys changed
dbfilename dump.rdb
dir /data/redis/
```

### Trigger Manual Snapshot

```bash
# Trigger a foreground save (blocks Redis)
redis-cli -h localhost -p 16379 -a helixagent123 SAVE

# Trigger a background save (non-blocking)
redis-cli -h localhost -p 16379 -a helixagent123 BGSAVE

# Check save status
redis-cli -h localhost -p 16379 -a helixagent123 LASTSAVE
```

### Copy RDB File

```bash
# Copy the RDB file to backup storage
cp /data/redis/dump.rdb /backups/redis/dump_$(date +%Y%m%d_%H%M%S).rdb
```

### AOF (Append Only File) Backup

For higher durability, enable AOF:

```
appendonly yes
appendfilename "appendonly.aof"
appendfsync everysec
```

Backup the AOF file:

```bash
cp /data/redis/appendonly.aof /backups/redis/aof_$(date +%Y%m%d_%H%M%S).aof
```

## Configuration Backup

### Full Configuration Backup

```bash
#!/bin/bash
# scripts/backup-config.sh
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups/config"
mkdir -p "${BACKUP_DIR}"

tar -czf "${BACKUP_DIR}/config_${TIMESTAMP}.tar.gz" \
    configs/ \
    .env \
    containers/.env \
    docker-compose*.yml \
    sql/schema/
```

### Sensitive Data Handling

The `.env` file contains API keys and secrets. Encrypt configuration backups:

```bash
# Encrypt with GPG
tar -czf - configs/ .env containers/.env | \
    gpg --symmetric --cipher-algo AES256 \
    -o "/backups/config/config_${TIMESTAMP}.tar.gz.gpg"

# Decrypt for recovery
gpg --decrypt "/backups/config/config_${TIMESTAMP}.tar.gz.gpg" | \
    tar -xzf -
```

## Vector Database Backup

### Qdrant Snapshots

```bash
# Create a Qdrant collection snapshot via API
curl -X POST "http://localhost:6333/collections/helixagent_embeddings/snapshots"

# List available snapshots
curl "http://localhost:6333/collections/helixagent_embeddings/snapshots"

# Download a snapshot
curl -o /backups/qdrant/snapshot.tar \
    "http://localhost:6333/collections/helixagent_embeddings/snapshots/<snapshot_name>"
```

### pgvector Backup

pgvector data is stored in PostgreSQL and is included in the standard `pg_dump` backup.

## Automated Backup Schedule

| Backup Type | Frequency | Retention | Method |
|---|---|---|---|
| PostgreSQL full dump | Daily at 02:00 | 30 days | `pg_dump --format=custom` |
| PostgreSQL WAL archive | Continuous | 7 days | WAL archiving |
| Redis RDB snapshot | Every 4 hours | 7 days | `BGSAVE` + copy |
| Configuration files | Daily at 03:00 | 90 days | `tar` + GPG encryption |
| Vector DB snapshots | Weekly (Sunday 04:00) | 4 weeks | API snapshots |

### Cron Configuration

```cron
# PostgreSQL daily backup
0 2 * * * /opt/helixagent/scripts/backup-database.sh >> /var/log/backup-db.log 2>&1

# Redis snapshot backup
0 */4 * * * /opt/helixagent/scripts/backup-redis.sh >> /var/log/backup-redis.log 2>&1

# Configuration backup
0 3 * * * /opt/helixagent/scripts/backup-config.sh >> /var/log/backup-config.log 2>&1

# Vector DB weekly snapshot
0 4 * * 0 /opt/helixagent/scripts/backup-vectordb.sh >> /var/log/backup-vectordb.log 2>&1
```

## Recovery Procedures

### PostgreSQL Full Recovery

```bash
# Stop HelixAgent to prevent writes during recovery
# (HelixAgent manages containers; stop the binary)

# Drop and recreate the database
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent -d postgres \
    -c "DROP DATABASE IF EXISTS helixagent_db;"
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent -d postgres \
    -c "CREATE DATABASE helixagent_db;"

# Restore from custom format dump
PGPASSWORD="${DB_PASSWORD}" pg_restore -h localhost -p 15432 -U helixagent \
    -d helixagent_db --no-owner --no-privileges \
    /backups/postgres/helixagent_db_20260308_020000.dump

# Restore from plain SQL
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent \
    -d helixagent_db < /backups/postgres/helixagent_db_20260308_020000.sql
```

### Redis Recovery

```bash
# Stop Redis
redis-cli -h localhost -p 16379 -a helixagent123 SHUTDOWN NOSAVE

# Replace the RDB file
cp /backups/redis/dump_20260308_020000.rdb /data/redis/dump.rdb

# Restart Redis (it loads the RDB on startup)
# (HelixAgent will restart containers automatically on next boot)
```

### Configuration Recovery

```bash
# Decrypt and extract
gpg --decrypt /backups/config/config_20260308_030000.tar.gz.gpg | tar -xzf -

# Verify the .env file contains correct values
cat .env | grep -c "API_KEY"
```

## Point-in-Time Recovery

Using PostgreSQL WAL archiving, recover to a specific timestamp:

```bash
# 1. Stop the PostgreSQL server
# 2. Create recovery.conf (or postgresql.auto.conf in PG 12+)
cat > /data/postgres/recovery.signal <<EOF
EOF

cat >> /data/postgres/postgresql.auto.conf <<EOF
restore_command = 'cp /backups/postgres/wal/%f %p'
recovery_target_time = '2026-03-08 10:00:00'
recovery_target_action = 'promote'
EOF

# 3. Start PostgreSQL - it will replay WAL to the target time
# 4. Verify data state
# 5. Remove recovery.signal when satisfied
```

## Backup Verification

### Automated Verification

```bash
#!/bin/bash
# scripts/verify-backup.sh
set -euo pipefail

BACKUP_FILE=$1
VERIFY_DB="helixagent_verify_$(date +%s)"

echo "Creating verification database: ${VERIFY_DB}"
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent -d postgres \
    -c "CREATE DATABASE ${VERIFY_DB};"

echo "Restoring backup..."
PGPASSWORD="${DB_PASSWORD}" pg_restore -h localhost -p 15432 -U helixagent \
    -d "${VERIFY_DB}" --no-owner "${BACKUP_FILE}"

echo "Verifying table counts..."
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent -d "${VERIFY_DB}" \
    -c "SELECT schemaname, tablename, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC;"

echo "Dropping verification database..."
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p 15432 -U helixagent -d postgres \
    -c "DROP DATABASE ${VERIFY_DB};"

echo "Backup verification complete."
```

Run verification after each backup:

```bash
./scripts/verify-backup.sh /backups/postgres/helixagent_db_20260308_020000.dump
```

## Retention Policies

| Data Type | Hot Storage | Warm Storage | Cold Storage |
|---|---|---|---|
| Database dumps | 7 days (local) | 30 days (S3) | 1 year (Glacier) |
| WAL archives | 7 days (local) | 14 days (S3) | N/A |
| Redis snapshots | 3 days (local) | 7 days (S3) | N/A |
| Config backups | 30 days (local) | 90 days (S3) | 1 year (Glacier) |
| Vector DB snapshots | 2 weeks (local) | 4 weeks (S3) | N/A |

## Backup Storage

### Local Storage

Default backup location: `/backups/` with subdirectories per service.

### S3-Compatible Storage

```bash
# Upload to S3
aws s3 cp /backups/postgres/helixagent_db_20260308_020000.dump \
    s3://helixagent-backups/postgres/

# Sync entire backup directory
aws s3 sync /backups/ s3://helixagent-backups/ --storage-class STANDARD_IA
```

### Remote Host via SCP

```bash
scp /backups/postgres/helixagent_db_20260308_020000.dump \
    backup-host:/backups/helixagent/postgres/
```

## Troubleshooting

### pg_dump Fails with "Too Many Connections"

**Symptom:** `pg_dump: error: connection to server failed: too many connections`

**Solutions:**
1. Increase `max_connections` in postgresql.conf
2. Close idle connections before backup
3. Use a dedicated backup user with reserved connection slots

### Redis BGSAVE Fails

**Symptom:** `Can't save in background: fork: Cannot allocate memory`

**Solutions:**
1. Set `vm.overcommit_memory = 1` in sysctl
2. Ensure available memory is at least 2x Redis dataset size
3. Reduce Redis memory usage with `maxmemory` and eviction policies

### Backup File is Corrupted

**Symptom:** `pg_restore: error: input file appears to be a text format dump`

**Solutions:**
1. Verify the backup was created with `--format=custom` for pg_restore
2. Check file integrity: `pg_restore --list backup.dump`
3. Use plain SQL format if custom format fails: `pg_dump > backup.sql`

### Recovery Takes Too Long

**Symptom:** Large database restore exceeds the RTO window.

**Solutions:**
1. Use parallel restore: `pg_restore --jobs=4`
2. Disable triggers during restore: `--disable-triggers`
3. Drop and recreate indexes after data load
4. Consider using PostgreSQL streaming replication instead of logical backups

## Related Resources

- [User Manual 29: Disaster Recovery](29-disaster-recovery.md) -- Full DR procedures and failover
- [User Manual 25: Multi-Region Deployment](25-multi-region-deployment.md) -- Cross-region replication
- [User Manual 30: Enterprise Architecture](30-enterprise-architecture.md) -- Production backup strategy
- Database schema: `sql/schema/`
- Database module: `Database/`
- PostgreSQL documentation: https://www.postgresql.org/docs/15/backup.html
