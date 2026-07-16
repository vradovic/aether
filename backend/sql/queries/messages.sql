-- name: InsertMessage :one
INSERT INTO messages (
    conversation_id,
    message_sequence,
    sender_id,
    body,
    client_message_id
)
SELECT $1, $2, $3, $4, $5
WHERE EXISTS (
    SELECT 1
    FROM conversation_participants
    WHERE conversation_id = $1 AND user_id = $3
)
RETURNING *;

-- name: SyncMessages :many
SELECT m.*
FROM messages m
INNER JOIN conversation_participants cp
    ON cp.conversation_id = m.conversation_id
        AND cp.user_id = sqlc.arg(user_id)
WHERE m.conversation_id = sqlc.arg(conversation_id)
    AND message_sequence > sqlc.arg(after_sequence)
ORDER BY m.message_sequence DESC
LIMIT sqlc.arg(page_size);
