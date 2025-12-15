CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid_v7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_active ON users(active);

-- ------------------------------------------------------------------------------

CREATE TABLE user_authentication_methods (
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,

    -- auth_type: 'password', 'oauth2_google', 'oauth2_github', etc.
    auth_type VARCHAR(50) NOT NULL,

    -- For password auth: stores bcrypt hashed password
    -- For OAuth2: stores provider user ID
    auth_identifier VARCHAR(255) NOT NULL,

    -- For OAuth2: stores access tokens, refresh tokens (encrypted)
    -- For password: NULL
    auth_metadata JSONB,

    PRIMARY KEY (user_id, auth_type)
);
CREATE INDEX idx_auth_methods_type_identifier ON user_authentication_methods(auth_type, auth_identifier);
