#!/bin/bash
set -e

# Create separate databases for the sample apps on the same PostgreSQL instance.
# This script runs as part of the Docker entrypoint before the SQL init scripts.

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'CREATE DATABASE banking OWNER quorum'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'banking')\gexec

    SELECT 'CREATE DATABASE expenses OWNER quorum'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'expenses')\gexec

    SELECT 'CREATE DATABASE access_portal OWNER quorum'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'access_portal')\gexec
EOSQL
