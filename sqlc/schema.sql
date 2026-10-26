CREATE TABLE user_accounts (
    email                   TEXT PRIMARY KEY,
    active                  BOOLEAN NOT NULL DEFAULT TRUE,
    roles                   TEXT[] NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports containment queries such as listing every user granted a role.
CREATE INDEX user_accounts_roles_idx ON user_accounts USING GIN (roles);
