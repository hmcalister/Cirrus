-- name: CreateUser :one
INSERT INTO user_accounts (email, password_hash, salt) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM user_accounts WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM user_accounts WHERE id = $1 LIMIT 1;

-- name: MarkVerified :exec
UPDATE user_accounts SET verified = TRUE WHERE id = $1;

-- name: SetActive :exec
UPDATE user_accounts SET active = $2 WHERE id = $1;

-- name: UpdatePassword :exec
UPDATE user_accounts SET password_hash = $2, salt = $3 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM user_accounts WHERE id = $1;

-- name: CreateUserToken :exec
INSERT INTO user_tokens (token_hash, user_id, purpose, expires_at) VALUES ($1, $2, $3, $4);

-- name: GetUserToken :one
SELECT user_id FROM user_tokens WHERE token_hash = $1 AND purpose = $2 AND expires_at > now() LIMIT 1;

-- Validates and burns the token in one statement.
-- name: ConsumeUserToken :one
DELETE FROM user_tokens WHERE token_hash = $1 AND purpose = $2 AND expires_at > now() RETURNING user_id;

-- name: DeleteUserTokensByPurpose :exec
DELETE FROM user_tokens WHERE user_id = $1 AND purpose = $2;

-- name: DeleteUserTokens :exec
DELETE FROM user_tokens WHERE user_id = $1;
