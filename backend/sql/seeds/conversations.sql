WITH new_conversation AS (
    INSERT INTO conversations (name, created_by)
        VALUES (
                   'Group Chat',
                   (SELECT id FROM users WHERE username = 'jdoe')
               )
        RETURNING id
)
INSERT INTO conversation_participants (conversation_id, user_id)
SELECT
    new_conversation.id,
    users.id
FROM new_conversation, users
WHERE users.username IN ('jdoe', 'jsmith', 'mgarcia');
