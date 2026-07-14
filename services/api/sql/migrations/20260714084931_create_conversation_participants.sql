-- +goose Up
CREATE TABLE conversation_participants (
    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX conversation_participants_user_idx
ON conversation_participants (user_id, conversation_id);

-- +goose Down
DROP INDEX conversation_participants_user_idx;
DROP TABLE conversation_participants;
