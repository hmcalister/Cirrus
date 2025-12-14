-- name: CreateUser :one
-- Creates a new user with email
INSERT INTO users (email)
VALUES ($1)
RETURNING *;

-- name: GetUserByID :one
-- Retrieves a user by their user_id
SELECT * FROM users
WHERE user_id = $1;

-- name: GetUserByEmail :one
-- Retrieves a user by their email address
SELECT * FROM users
WHERE email = $1;

-- name: SetUserActive :one
-- Sets a user's active status
UPDATE users
SET active = $2, updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: DeleteUser :one
-- Delete a user
DELETE FROM users
WHERE user_id = $1
RETURNING *;

-- ------------------------------------------------------------------------------

-- name: CreateAuthenticationMethod :one
-- Creates a new authentication method for a user
INSERT INTO user_authentication_methods (user_id, auth_type, auth_identifier, auth_metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAuthenticationMethod :one
-- Retrieves a specific authentication method for a user
SELECT * FROM user_authentication_methods
WHERE user_id = $1 AND auth_type = $2;

-- name: GetAuthenticationMethodsByUserID :many
-- Retrieves all authentication methods for a user
SELECT * FROM user_authentication_methods
WHERE user_id = $1;

-- name: GetUserByAuthTypeAndIdentifier :one
-- Finds a user by their authentication type and identifier
-- Used during login to find which user owns this auth credential
SELECT u.* FROM users u
INNER JOIN user_authentication_methods uam ON u.user_id = uam.user_id
WHERE uam.auth_type = $1 AND uam.auth_identifier = $2;

-- name: UpdateAuthenticationIdentifier :one
-- Updates the auth_identifier for a user's authentication method (e.g., password change)
UPDATE user_authentication_methods
SET auth_identifier = $3
WHERE user_id = $1 AND auth_type = $2
RETURNING *;

-- name: UpdateAuthenticationMetadata :one
-- Updates the auth_metadata for a user's authentication method (e.g., OAuth2 tokens)
UPDATE user_authentication_methods
SET auth_metadata = $3
WHERE user_id = $1 AND auth_type = $2
RETURNING *;

-- name: DeleteAuthenticationMethod :exec
-- Removes an authentication method from a user
DELETE FROM user_authentication_methods
WHERE user_id = $1 AND auth_type = $2;

-- name: DeleteAllAuthenticationMethods :exec
-- Removes all authentication methods for a user (used during account deletion)
DELETE FROM user_authentication_methods
WHERE user_id = $1;