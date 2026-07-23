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
WHERE cp.user_id = $1
ORDER BY c.updated_at DESC, c.id;

-- name: CreateConversationWithCreator :one
WITH inserted_conversation AS (
    INSERT INTO conversations (name, created_by)
    VALUES ($1, $2)
    RETURNING *
), inserted_participant AS (
    INSERT INTO conversation_participants (conversation_id, user_id)
    SELECT id, created_by
    FROM inserted_conversation
    RETURNING conversation_id
)
SELECT inserted_conversation.*
FROM inserted_conversation
INNER JOIN inserted_participant
    ON inserted_participant.conversation_id = inserted_conversation.id;

-- name: InsertConversation :one
INSERT INTO conversations (name, created_by)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateConversationName :one
UPDATE conversations
SET name = $1, updated_at = now()
WHERE id = $2
  AND created_by = $3
RETURNING *;

-- name: DeleteConversation :one
DELETE FROM conversations
WHERE id = $1
  AND created_by = $2
RETURNING id;

-- name: InsertConversationParticipant :one
INSERT INTO conversation_participants (conversation_id, user_id)
SELECT $1, $2
WHERE EXISTS (
    SELECT 1
    FROM conversations
    WHERE id = $1
        AND created_by = $3
)
AND EXISTS (
    SELECT 1
    FROM contacts
    WHERE user1_id = LEAST($2, $3)
      AND user2_id = GREATEST($2, $3)
)
RETURNING *;

-- name: DeleteConversationParticipant :one
DELETE FROM conversation_participants cp
USING conversations c
WHERE cp.conversation_id = $1
  AND cp.user_id = $2
  AND c.id = cp.conversation_id
  AND c.created_by = $3
  AND cp.user_id <> c.created_by
RETURNING cp.*;

-- name: IsConversationOwner :one
SELECT EXISTS (
    SELECT 1
    FROM conversations
    WHERE id = $1 AND created_by = $2
);

-- name: AreContacts :one
SELECT EXISTS (
    SELECT 1
    FROM contacts
    WHERE user1_id = LEAST(sqlc.arg(user_id)::uuid, sqlc.arg(contact_id)::uuid)
      AND user2_id = GREATEST(sqlc.arg(user_id)::uuid, sqlc.arg(contact_id)::uuid)
);

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
