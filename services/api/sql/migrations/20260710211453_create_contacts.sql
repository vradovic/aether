-- +goose Up
CREATE TABLE contacts (
    user1 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user2 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (user1, user2)
);

-- +goose Down
DROP TABLE contacts;
