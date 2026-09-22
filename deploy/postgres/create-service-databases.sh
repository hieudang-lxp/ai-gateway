#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 <<'SQL'
SELECT 'CREATE DATABASE gateway' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'gateway')
\gexec
SELECT 'CREATE DATABASE collector' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'collector')
\gexec
SELECT 'CREATE DATABASE sessions' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'sessions')
\gexec
SELECT 'CREATE DATABASE insights' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'insights')
\gexec
SELECT 'CREATE DATABASE memory_sync' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'memory_sync')
\gexec
SQL
