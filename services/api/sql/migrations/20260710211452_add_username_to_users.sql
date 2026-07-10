-- +goose Up
ALTER TABLE users
ADD COLUMN username TEXT NOT NULL UNIQUE
    CONSTRAINT users_username_length_check
    CHECK (char_length(username) BETWEEN 3 AND 30);

-- +goose Down
ALTER TABLE users
DROP COLUMN username;
