-- +goose Up
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    conversation_id UUID NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL
        REFERENCES users(id),

    client_message_id UUID NOT NULL, -- client generated
    UNIQUE (sender_id, client_message_id),

    message_sequence BIGINT NOT NULL,

    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 500),

    UNIQUE (conversation_id, message_sequence),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT message_sequence_non_negative
        CHECK (message_sequence > 0)
);

-- +goose Down
DROP TABLE messages;
