-- name: NextMessageSequence :one
UPDATE conversations
SET last_message_sequence = last_message_sequence + 1
WHERE id = $1
RETURNING last_message_sequence;

-- name: GetConversationsForUser :many
SELECT c.*
FROM conversations c
INNER JOIN conversation_participants cp
    ON c.id = cp.conversation_id
WHERE cp.user_id = $1;

-- name: InsertConversation :one
INSERT INTO conversations (name, created_by)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateConversationName :one
UPDATE conversations
SET name = $1, updated_at = now()
WHERE id = $2
RETURNING *;

-- name: DeleteConversation :exec
DELETE FROM conversations
WHERE id = $1;

-- name: InsertConversationParticipant :one
INSERT INTO conversation_participants (conversation_id, user_id)
SELECT $1, $2
WHERE EXISTS (
    SELECT 1
    FROM conversations
    WHERE id = $1
        AND created_by = $3
)
RETURNING *;

-- name: IsConversationParticipant :one
SELECT EXISTS (
    SELECT 1
    FROM conversation_participants
    WHERE conversation_id = $1 AND user_id = $2
);

-- name: GetConversationRecipientIDs :many
SELECT user_id
FROM conversation_participants
WHERE conversation_id = $1
ORDER BY user_id;
