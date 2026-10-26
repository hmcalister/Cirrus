-- Returns the profile for the Stratus identity, creating it on first sign in.
-- The no-op UPDATE returns the existing row atomically, including when two
-- first requests race, so callers always get a profile back.
-- name: GetOrCreateUser :one
INSERT INTO user_accounts (email) VALUES ($1)
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING *;

-- name: SetActive :exec
UPDATE user_accounts SET active = $2 WHERE email = $1;

-- name: DeleteUser :exec
DELETE FROM user_accounts WHERE email = $1;

-- Returns the address of every user who still receives daily emails.
-- name: ListActiveEmails :many
SELECT email FROM user_accounts WHERE active ORDER BY email;
