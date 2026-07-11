-- +goose Up
ALTER TABLE contacts
ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
ADD CONSTRAINT contacts_distinct_users_check CHECK (user1 <> user2),
ADD CONSTRAINT contacts_canonical_order_check CHECK (user1 < user2);

CREATE TABLE contact_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (sender_id <> recipient_id)
);

CREATE UNIQUE INDEX contact_requests_one_pending_pair_idx
ON contact_requests (
    LEAST(sender_id, recipient_id),
    GREATEST(sender_id, recipient_id)
)
WHERE status = 'pending';

-- +goose Down
DROP TABLE contact_requests;

ALTER TABLE contacts
DROP CONSTRAINT contacts_canonical_order_check,
DROP CONSTRAINT contacts_distinct_users_check,
DROP COLUMN updated_at,
DROP COLUMN created_at;
