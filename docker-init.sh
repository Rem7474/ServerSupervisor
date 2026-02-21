#!/bin/bash
set -e

# PostgreSQL init script - this runs as root before postgres starts
# Create the database if it doesn't exist

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "postgres" <<-EOSQL
    SELECT 'CREATE DATABASE serversupervisor' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'serversupervisor')\gexec
EOSQL

# Run migrations on serversupervisor database
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "serversupervisor" <<-EOSQL
    -- Add agent_version column if it doesn't exist
    ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;
    
    -- Create index for faster lookups by version
    CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);
    
    -- Add cve_list column to apt_status table
    ALTER TABLE apt_status ADD COLUMN IF NOT EXISTS cve_list TEXT DEFAULT '[]';
    
    -- Update existing rows to have empty JSON array
    UPDATE apt_status SET cve_list = '[]' WHERE cve_list IS NULL;
EOSQL

echo "Database initialization and migrations complete"

