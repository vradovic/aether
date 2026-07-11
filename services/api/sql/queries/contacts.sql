-- name: SendContactRequest :one
INSERT INTO contact_requests (sender_id, recipient_id)
SELECT sqlc.arg(sender_id), id
FROM users
WHERE username = sqlc.arg(username)
RETURNING id;

-- name: CancelContactRequest :one
UPDATE contact_requests
SET status = 'cancelled', updated_at = now()
WHERE id = sqlc.arg(request_id)
  AND sender_id = sqlc.arg(sender_id)
  AND status = 'pending'
RETURNING id;

-- name: DeclineContactRequest :one
UPDATE contact_requests
SET status = 'declined', updated_at = now()
WHERE id = sqlc.arg(request_id)
  AND recipient_id = sqlc.arg(recipient_id)
  AND status = 'pending'
RETURNING id;

-- name: AcceptContactRequest :one
WITH accepted AS (
    UPDATE contact_requests
    SET status = 'accepted', updated_at = now()
    WHERE id = sqlc.arg(request_id)
      AND recipient_id = sqlc.arg(recipient_id)
      AND status = 'pending'
    RETURNING sender_id, recipient_id
), inserted AS (
    INSERT INTO contacts (user1, user2)
    SELECT
        LEAST(sender_id, recipient_id),
        GREATEST(sender_id, recipient_id)
    FROM accepted
    ON CONFLICT DO NOTHING
)
SELECT EXISTS(SELECT 1 FROM accepted) AS accepted;
