#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE orders_db;
    CREATE DATABASE restaurant_db;
    CREATE DATABASE delivery_db;
    CREATE DATABASE notification_db;
EOSQL
