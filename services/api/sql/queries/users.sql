-- name: GetUserByEmail :one
SELECT
    id,
    email,
    first_name,
    last_name
FROM users
WHERE email = $1;
