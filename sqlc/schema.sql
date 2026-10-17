CREATE TABLE user_accounts (
    id                      UUID PRIMARY KEY DEFAULT uuidv7(),
    email                   TEXT NOT NULL UNIQUE,
    password_hash           BYTEA NOT NULL,
    salt                    BYTEA NOT NULL,
    verified                BOOLEAN DEFAULT FALSE,
    active                  BOOLEAN DEFAULT TRUE,
    created_at              TIMESTAMPTZ
);

CREATE TABLE user_tokens (
    token_hash              BYTEA PRIMARY KEY,
    user_id                 UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    purpose                 TEXT NOT NULL,
    expires_at              TIMESTAMPTZ NOT NULL
)
