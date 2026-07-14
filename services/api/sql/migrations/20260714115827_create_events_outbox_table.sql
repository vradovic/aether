-- +goose Up
CREATE TABLE events_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'message.created'
    )),
    subject TEXT NOT NULL,
    payload JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,

    CONSTRAINT events_outbox_payload_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX events_outbox_pending_idx
ON events_outbox (created_at, id)
WHERE published_at IS NULL;

-- +goose Down
DROP TABLE events_outbox;
