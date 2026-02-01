#!/bin/bash
set -e

# Enable more verbose error reporting
set -o pipefail

echo "========================================"
echo "PostgreSQL 18 Master Setup Script"
echo "Authentication: SCRAM-SHA-256"
echo "========================================"

# Function for logging with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function for error handling
error_exit() {
    log "ERROR: $1"
    exit 1
}

log "Waiting for PostgreSQL to be ready..."

# Wait for PostgreSQL to be ready with timeout
TIMEOUT=60
COUNTER=0
until pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
    if [ $COUNTER -ge $TIMEOUT ]; then
        error_exit "PostgreSQL failed to start within $TIMEOUT seconds"
    fi
    log "PostgreSQL not ready yet, waiting... ($COUNTER/$TIMEOUT)"
    sleep 2
    COUNTER=$((COUNTER + 2))
done

log "PostgreSQL is ready! Waiting for full initialization..."
sleep 5

# Check if readonly_user already exists
log "Checking if readonly_user exists..."
if psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c "SELECT 1 FROM pg_roles WHERE rolname = 'readonly_user';" | grep -q 1; then
    log "readonly_user already exists, skipping user creation..."
else
    log "Creating readonly user with SCRAM-SHA-256 authentication..."

    # Create readonly user with SCRAM-SHA-256 password
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        -- Create readonly user with SCRAM-SHA-256 encryption and replication permissions
        SET password_encryption = 'scram-sha-256';
        CREATE USER readonly_user WITH PASSWORD 'password' REPLICATION;

        -- Grant necessary permissions for read-only access
        GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO readonly_user;
        GRANT USAGE ON SCHEMA public TO readonly_user;
        GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;
        GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO readonly_user;

        -- Set default privileges for future tables
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO readonly_user;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO readonly_user;
EOSQL
    log "readonly_user created successfully!"
fi

# Check if SCRAM-SHA-256 replication permissions are configured in pg_hba.conf
log "Checking pg_hba.conf replication configuration..."
if grep -q "host.*replication.*all.*0\.0\.0\.0/0.*scram-sha-256" "$PGDATA/pg_hba.conf"; then
    log "SCRAM-SHA-256 replication permissions already configured, skipping..."
else
    log "Updating pg_hba.conf with SCRAM-SHA-256 replication permissions..."

    # Remove old md5 entries if they exist
    if grep -q "host.*replication.*all.*0\.0\.0\.0/0.*md5" "$PGDATA/pg_hba.conf"; then
        log "Removing old md5 replication entries..."
        sed -i '/host.*replication.*all.*0\.0\.0\.0\/0.*md5/d' "$PGDATA/pg_hba.conf"
        sed -i '/host.*all.*all.*0\.0\.0\.0\/0.*md5/d' "$PGDATA/pg_hba.conf"
    fi

    # Add SCRAM-SHA-256 replication permissions to pg_hba.conf
    cat >> "$PGDATA/pg_hba.conf" <<EOF

# Allow replication connections with SCRAM-SHA-256 authentication
host    replication     all             0.0.0.0/0               scram-sha-256
host    all             all             0.0.0.0/0               scram-sha-256
EOF

    # Reload PostgreSQL configuration
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "SELECT pg_reload_conf();"
    log "pg_hba.conf updated with SCRAM-SHA-256 and reloaded!"
fi

# Check if replication slot already exists
log "Checking replication slot configuration..."
if psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c "SELECT 1 FROM pg_replication_slots WHERE slot_name = 'replica_slot';" | grep -q 1; then
    log "Replication slot 'replica_slot' already exists, skipping creation..."
else
    log "Creating replication slot 'replica_slot'..."

    # Create replication slot
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        SELECT pg_create_physical_replication_slot('replica_slot');
EOSQL
    log "Replication slot created successfully!"
fi

log "PostgreSQL 18 master setup completed successfully!"
echo ""
echo "=== Current Configuration Summary ==="
echo "Authentication Method: SCRAM-SHA-256"
echo "Replication Slot: replica_slot"
echo ""
echo "=== Active Replication Slots ==="
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT slot_name, active, restart_lsn, confirmed_flush_lsn FROM pg_replication_slots;"
echo ""
echo "=== User Authentication Methods ==="
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT rolname, CASE WHEN rolpassword LIKE 'SCRAM-SHA-256%' THEN 'SCRAM-SHA-256' WHEN rolpassword LIKE 'md5%' THEN 'MD5' ELSE 'OTHER' END as auth_method FROM pg_authid WHERE rolname IN ('$POSTGRES_USER', 'readonly_user');"
echo ""
echo "=== Setup Complete ==="
