-- Create organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    name VARCHAR(100) NOT NULL
);

CREATE INDEX idx_organizations_name ON organizations(name);

-- Create certificates table
CREATE TABLE IF NOT EXISTS certificates (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    name VARCHAR(250) NOT NULL,
    is_ca BOOLEAN NOT NULL,
    is_local BOOLEAN NOT NULL,
    type VARCHAR(250) NOT NULL,
    source VARCHAR(250) NOT NULL,
    comments VARCHAR(500),
    cert_bundle BYTEA,
    ref_count INTEGER
);

CREATE INDEX idx_certificates_name ON certificates(name);
CREATE INDEX idx_certificates_type ON certificates(type);

-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    name VARCHAR(250) NOT NULL,
    description VARCHAR(16000)
);

CREATE INDEX idx_roles_name ON roles(name);

-- Create role_permissions table
CREATE TABLE IF NOT EXISTS role_permissions (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    role_id INTEGER NOT NULL,
    permission VARCHAR(250) NOT NULL
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);

-- Create contacts table
CREATE TABLE IF NOT EXISTS contacts (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    email VARCHAR(500),
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    organization_id INTEGER,
    title VARCHAR(50)
);

CREATE INDEX idx_contacts_email ON contacts(email);
CREATE INDEX idx_contacts_organization_id ON contacts(organization_id);

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    erased BOOLEAN NOT NULL DEFAULT FALSE,
    hidden BOOLEAN,
    version BIGINT,
    username VARCHAR(250) NOT NULL,
    password VARCHAR(60) NOT NULL,
    organization_id INTEGER NOT NULL,
    role VARCHAR(250) NOT NULL,
    require_password_reset BOOLEAN NOT NULL,
    failed_logins_count INTEGER NOT NULL DEFAULT 0,
    expired BOOLEAN NOT NULL,
    locked BOOLEAN NOT NULL,
    pass_can_expired BOOLEAN NOT NULL,
    disabled BOOLEAN NOT NULL,
    contact_id INTEGER
);

CREATE UNIQUE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_users_contact_id ON users(contact_id);
