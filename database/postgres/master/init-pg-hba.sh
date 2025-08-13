#!/bin/bash
set -e

echo "Checking master replication setup..."

# Wait for PostgreSQL to be ready
until pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"; do
    echo "Waiting for PostgreSQL to be ready..."
    sleep 2
done

# Wait a bit more for PostgreSQL to fully initialize
sleep 5

# Check if readonly_user already exists
if psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c "SELECT 1 FROM pg_roles WHERE rolname = 'readonly_user';" | grep -q 1; then
    echo "readonly_user already exists, skipping user creation..."
else
    echo "Creating readonly user..."
    
    # Create readonly user
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        CREATE USER readonly_user WITH PASSWORD 'password';
        
        -- Grant necessary permissions
        GRANT CONNECT ON DATABASE auth_db TO readonly_user;
        GRANT USAGE ON SCHEMA public TO readonly_user;
        GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;
        GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO readonly_user;
        
        -- Set default privileges for future tables
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO readonly_user;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO readonly_user;
EOSQL
    echo "readonly_user created successfully!"
fi

# Check if replication permissions are already configured in pg_hba.conf
if grep -q "host.*replication.*all.*0\.0\.0\.0/0.*md5" "$PGDATA/pg_hba.conf"; then
    echo "Replication permissions already configured in pg_hba.conf, skipping..."
else
    echo "Updating pg_hba.conf with replication permissions..."
    
    # Add replication permissions to pg_hba.conf
    cat >> "$PGDATA/pg_hba.conf" <<EOF

# Allow replication connections from replica container
host    replication     all             0.0.0.0/0               md5
host    all             all             0.0.0.0/0               md5
EOF

    # Reload PostgreSQL configuration
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "SELECT pg_reload_conf();"
    echo "pg_hba.conf updated and reloaded!"
fi

# Check if replication slot already exists
if psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c "SELECT 1 FROM pg_replication_slots WHERE slot_name = 'replica_slot';" | grep -q 1; then
    echo "Replication slot 'replica_slot' already exists, skipping creation..."
else
    echo "Creating replication slot..."
    
    # Create replication slot
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        SELECT pg_create_physical_replication_slot('replica_slot');
EOSQL
    echo "Replication slot created successfully!"
fi

echo "Master replication setup completed successfully!"
echo "Current replication status:"
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT slot_name, active, restart_lsn FROM pg_replication_slots;"
