-- +goose Up
CREATE TABLE contacts (
    user1_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user2_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (user1_id, user2_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    CONSTRAINT contacts_distinct_users_check CHECK (user1_id <> user2_id),
    CONSTRAINT contacts_canonical_order_check CHECK (user1_id < user2_id)
);

CREATE INDEX contacts_user2_user1_idx
ON contacts (user2_id, user1_id);

-- +goose Down
DROP TABLE contacts;
