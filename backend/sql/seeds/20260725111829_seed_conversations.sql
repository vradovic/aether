-- +goose Up
INSERT INTO conversations (id, name, created_by)
    VALUES ('00000000-0000-0000-0000-000000000001', 'Group Chat', '00000000-0000-0000-0000-000000000001');

INSERT INTO conversation_participants (conversation_id, user_id)
SELECT '00000000-0000-0000-0000-000000000001', users.id
FROM users;
