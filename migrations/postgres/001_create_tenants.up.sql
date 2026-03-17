CREATE SCHEMA IF NOT EXISTS quorum;

CREATE TABLE IF NOT EXISTS quorum.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO quorum.tenants (slug, name) VALUES ('default', 'Default Tenant');
