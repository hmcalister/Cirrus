-- name: UpsertUser :one
-- Creates a new user with email. If the email already exists, do nothing.
INSERT INTO users (email)
VALUES ($1)
ON CONFLICT (email)
DO UPDATE SET
    updated_at = NOW()
RETURNING *;

-- name: GetUserByID :one
-- Retrieves a user by their user_id.
SELECT * FROM users
WHERE user_id = $1;

-- name: GetUserByEmail :one
-- Retrieves a user by their email address.
SELECT * FROM users
WHERE email = $1;

-- name: GetRecipientUsers :many
-- Retrieves all users who are to receive a cloud email.
SELECT * FROM users
WHERE validated=TRUE AND active=TRUE;

-- name: SetUserValidatedStatus :one
-- Sets a user's validated status
-- A user is validated when we are sure they can receive emails to their inbox.
-- An invalid user will never receive emails.
UPDATE users
SET validated = $2, updated_at = NOW()
WHERE email = $1
RETURNING *;

-- name: SetUserActiveStatus :one
-- Sets a user's active status.
-- An active user is one that will receive emails (assuming they are also validated).
UPDATE users
SET active = $2, updated_at = NOW()
WHERE email = $1
RETURNING *;

-- name: DeleteUser :one
-- Delete a user.
DELETE FROM users
WHERE user_id = $1
RETURNING *;

-- ------------------------------------------------------------------------------
-- Password authentication methods

-- name: CreatePasswordAuthentication :one
-- Creates a new password authentication method for a user.
INSERT INTO user_authentication_password (email, hashed_password, salt)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPasswordAuthentication :one
-- Retrieves the hashed password and salt for a user.
SELECT * FROM user_authentication_password
WHERE email = $1;

-- name: UpdatePassword :one
-- Updates the password and salt for a user.
UPDATE user_authentication_password
SET hashed_password = $2, salt = $3
WHERE email = $1
RETURNING *;

-- name: DeletePasswordAuthentication :exec
-- Removes password authentication from a user.
DELETE FROM user_authentication_password
WHERE email = $1;

-- ------------------------------------------------------------------------------
-- OAuth2 authentication methods

-- name: CreateOAuth2Authentication :one
-- Creates a new OAuth2 authentication method for a user.
INSERT INTO user_authentication_oauth2 (user_id, oauth2_provider, provider_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOAuth2Authentication :one
-- Retrieves a specific OAuth2 authentication method for a user.
SELECT * FROM user_authentication_oauth2
WHERE user_id = $1 AND oauth2_provider = $2;

-- name: UpdateOAuth2Authentication :one
-- Updates the oauth2 authentication for a user.
UPDATE user_authentication_oauth2
SET oauth2_provider = $2, provider_user_id = $3
WHERE user_id = $1
RETURNING *;

-- name: DeleteOAuth2Authentication :exec
-- Removes an OAuth2 authentication method from a user.
DELETE FROM user_authentication_oauth2
WHERE user_id = $1 AND oauth2_provider = $2;

-- name: DeleteAllOAuth2Authentication :exec
-- Removes all OAuth2 authentication methods for a user.
DELETE FROM user_authentication_oauth2
WHERE user_id = $1;