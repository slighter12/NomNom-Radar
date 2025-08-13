#!/bin/bash
set -e

echo "Configuring master replication setup..."

# Wait for PostgreSQL to be ready
until pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"; do
    echo "Waiting for PostgreSQL to be ready..."
    sleep 2
done

# Wait a bit more for PostgreSQL to fully initialize
sleep 5

echo "Creating readonly user..."

# Create readonly user if it doesn't exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create readonly user if it doesn't exist
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'readonly_user') THEN
            CREATE USER readonly_user WITH PASSWORD 'password';
        END IF;
    END
    \$\$;
    
    -- Grant necessary permissions
    GRANT CONNECT ON DATABASE auth_db TO readonly_user;
    GRANT USAGE ON SCHEMA public TO readonly_user;
    GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;
    GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO readonly_user;
    
    -- Set default privileges for future tables
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO readonly_user;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO readonly_user;
EOSQL

echo "Updating pg_hba.conf with replication permissions..."

# Add replication permissions to pg_hba.conf
cat >> "$PGDATA/pg_hba.conf" <<EOF

# Allow replication connections from replica container
host    replication     all             0.0.0.0/0               md5
host    all             all             0.0.0.0/0               md5
EOF

# Reload PostgreSQL configuration
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "SELECT pg_reload_conf();"

echo "Creating replication slot..."

# Create replication slot if it doesn't exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create replication slot if it doesn't exist
    SELECT pg_create_physical_replication_slot('replica_slot') 
    WHERE NOT EXISTS (
        SELECT 1 FROM pg_replication_slots WHERE slot_name = 'replica_slot'
    );
EOSQL

echo "Master replication setup completed successfully!"
