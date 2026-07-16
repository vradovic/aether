-- +goose Up
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT CHECK (char_length(name) BETWEEN 1 AND 50),
    created_by UUID NOT NULL REFERENCES users(id),
    last_message_sequence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT message_sequence_non_negative
        CHECK (last_message_sequence >= 0)
);

-- +goose Down
DROP TABLE conversations;
