CREATE TYPE user_role as ENUM('user', 'admin');

CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    account_role user_role NOT NULL DEFAULT 'user',
    -- Flag to set whether this email address has been validated.
    -- An email address is validated by checking if the user can access it (sending a validation link).
    validated BOOLEAN NOT NULL DEFAULT FALSE,
    -- Flag as to whether the user is receiving emails.
    -- Note that an non-validated user (validated flag set to false) will still not receive emails.
    active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_receivers ON users(validated, active);

-- ------------------------------------------------------------------------------

CREATE TABLE user_authentication_password (
    email VARCHAR(255) PRIMARY KEY REFERENCES users(email) ON DELETE CASCADE,
    hashed_password BYTEA NOT NULL,
    salt BYTEA NOT NULL
);

CREATE TABLE user_authentication_oauth2 (
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,

    -- 'google', 'github', etc.
    oauth2_provider VARCHAR(32) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,

    PRIMARY KEY (user_id, oauth2_provider)
);
CREATE INDEX idx_user_authentication_oauth2_provider_info ON user_authentication_oauth2(oauth2_provider, provider_user_id);
