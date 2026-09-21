#!/bin/sh
set -eu
# psql's gexec executes CREATE DATABASE outside a transaction. Keep fixed names
# aligned with the service DATABASE_URLs; existing databases are left intact.
psql -v ON_ERROR_STOP=1 <<'SQL'
SELECT 'CREATE DATABASE gateway' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'gateway')
\gexec
SELECT 'CREATE DATABASE collector' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'collector')
\gexec
SELECT 'CREATE DATABASE sessions' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'sessions')
\gexec
SELECT 'CREATE DATABASE insights' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'insights')
\gexec
SQL
