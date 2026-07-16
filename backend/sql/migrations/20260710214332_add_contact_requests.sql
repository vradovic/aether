-- +goose Up
CREATE TABLE contact_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending',
        'accepted',
        'declined',
        'cancelled'
    )),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT contact_requests_distinct_users_check CHECK (sender_id <> recipient_id)
);

CREATE UNIQUE INDEX contact_requests_one_pending_pair_idx
ON contact_requests (
    LEAST(sender_id, recipient_id),
    GREATEST(sender_id, recipient_id)
)
WHERE status = 'pending';

-- +goose StatementBegin
CREATE FUNCTION prevent_request_if_contacts()
RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM contacts
        WHERE user1_id = LEAST(NEW.sender_id, NEW.recipient_id)
          AND user2_id = GREATEST(NEW.sender_id, NEW.recipient_id)
    ) THEN
        RAISE EXCEPTION 'users are already contacts';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER contact_requests_existing_contacts_trigger
BEFORE INSERT ON contact_requests
FOR EACH ROW
EXECUTE FUNCTION prevent_request_if_contacts();

-- +goose Down
DROP TRIGGER contact_requests_existing_contacts_trigger ON contact_requests;
DROP FUNCTION prevent_request_if_contacts();
DROP INDEX contact_requests_one_pending_pair_idx;
DROP TABLE contact_requests;
