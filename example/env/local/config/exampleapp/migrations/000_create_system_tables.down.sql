DROP INDEX IF EXISTS idx_users_contact_id;
DROP INDEX IF EXISTS idx_users_organization_id;
DROP INDEX IF EXISTS idx_users_username;
DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_contacts_organization_id;
DROP INDEX IF EXISTS idx_contacts_email;
DROP TABLE IF EXISTS contacts;

DROP INDEX IF EXISTS idx_role_permissions_role_id;
DROP TABLE IF EXISTS role_permissions;

DROP INDEX IF EXISTS idx_roles_name;
DROP TABLE IF EXISTS roles;

DROP INDEX IF EXISTS idx_certificates_type;
DROP INDEX IF EXISTS idx_certificates_name;
DROP TABLE IF EXISTS certificates;

DROP INDEX IF EXISTS idx_organizations_name;
DROP TABLE IF EXISTS organizations;
