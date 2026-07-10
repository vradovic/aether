-- name: GetUserByEmail :one
SELECT
    id,
    email,
    first_name,
    last_name
FROM users
WHERE email = $1;

-- name: CreateUser :exec
INSERT INTO users (email, password_hash, first_name, last_name)
VALUES ($1, $2, $3, $4);
