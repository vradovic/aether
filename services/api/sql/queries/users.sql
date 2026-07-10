-- name: GetUserByEmail :one
SELECT
    id,
    email,
    username,
    first_name,
    last_name
FROM users
WHERE email = $1;

-- name: GetUserCredentialsByEmail :one
SELECT
    id AS user_id,
    password_hash
FROM users
WHERE email = $1;

-- name: CreateUser :exec
INSERT INTO users (email, username, password_hash, first_name, last_name)
VALUES ($1, $2, $3, $4, $5);
