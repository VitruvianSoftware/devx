-- Migration: 001_initial.sql
-- Project: /var/folders/cr/4ch4t4hd0nd1t8tk23km89h80000gn/T/TestScaffoldCommand806656599/002/my-app
-- Created: 2026
--
-- This is the initial database schema.
-- Run migrations with: psql $DATABASE_URL -f db/migrations/001_initial.sql

BEGIN;

-- Enable useful Postgres extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Example table — replace with your actual schema
CREATE TABLE IF NOT EXISTS items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS items_name_trgm ON items USING GIN (name gin_trgm_ops);

-- Automatically update updated_at on row change
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER items_set_updated_at
    BEFORE UPDATE ON items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
