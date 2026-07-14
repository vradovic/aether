-- name: InsertOutboxEvent :exec
INSERT INTO events_outbox (
    event_type,
    subject,
    payload
)
VALUES ($1, $2, $3);

-- name: GetPendingOutboxEvents :many
SELECT id, event_type, subject, payload
FROM events_outbox
WHERE published_at IS NULL
ORDER BY created_at, id
LIMIT $1;

-- name: MarkPublishedOutboxEvents :execrows
UPDATE events_outbox
SET published_at = now()
WHERE id = ANY(sqlc.arg(ids)::bigint[]) AND published_at IS NULL;
