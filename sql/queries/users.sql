-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: Reset :exec
DELETE FROM users;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUserByID :one
UPDATE users SET updated_at = NOW(), hashed_password = $1, email = $2 WHERE id = $3 RETURNING *;

-- name: UpgradeUserRed :exec
UPDATE users SET updated_at = NOW(), is_chirpy_red = TRUE WHERE id = $1;
