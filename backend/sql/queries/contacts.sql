-- name: SendContactRequest :one
INSERT INTO contact_requests (sender_id, recipient_id)
SELECT $1, id
FROM users
WHERE username = $2
RETURNING *;

-- name: GetPendingContactRequests :many
SELECT *
FROM contact_requests
WHERE recipient_id = $1
  AND status = 'pending'
ORDER BY created_at DESC;

-- name: CancelContactRequest :one
UPDATE contact_requests
SET status = 'cancelled', updated_at = now()
WHERE id = $1
  AND sender_id = $2
  AND status = 'pending'
RETURNING *;

-- name: DeclineContactRequest :one
UPDATE contact_requests
SET status = 'declined', updated_at = now()
WHERE id = $1
  AND recipient_id = $2
  AND status = 'pending'
RETURNING *;

-- use AcceptContactRequest and InsertContact in transaction
-- name: AcceptContactRequest :one
UPDATE contact_requests
SET status = 'accepted', updated_at = now()
WHERE id = $1
  AND recipient_id = $2
  AND status = 'pending'
RETURNING *;

-- name: InsertContact :one
INSERT INTO contacts (user1_id, user2_id)
VALUES (
  LEAST($1, $2),
  GREATEST($1, $2)
)
RETURNING *;

-- name: GetContacts :many
SELECT u.id, u.username, u.first_name, u.last_name
FROM contacts c
INNER JOIN users u
  ON u.id = CASE
    WHEN c.user1_id = $1 THEN c.user2_id
    ELSE c.user1_id
  END
WHERE user1_id = $1 OR user2_id = $1;

-- name: DeleteContact :exec
DELETE FROM contacts
WHERE user1_id = LEAST($1, $2)
  AND user2_id = GREATEST($1, $2);
