-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, hashed_password, email)
VALUES (
  gen_random_uuid(),
  NOW(),
  NOW(),
  $1,
  $2
)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: LogUserIn :one
SELECT * FROM users
WHERE email = $1;
