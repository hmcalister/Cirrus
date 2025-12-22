CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP
);
CREATE INDEX idx_users_active ON users(active);

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
