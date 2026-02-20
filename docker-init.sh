#!/bin/bash
set -e

# PostgreSQL init script - this runs as root before postgres starts
# Create the database if it doesn't exist

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "postgres" <<-EOSQL
    SELECT 'CREATE DATABASE serversupervisor' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'serversupervisor')\gexec
EOSQL

echo "Database initialization complete"

