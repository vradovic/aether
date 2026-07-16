-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE
        CHECK (char_length(username) BETWEEN 2 AND 50),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL
        CHECK (char_length(first_name) BETWEEN 2 AND 50),
    last_name TEXT NOT NULL
        CHECK (char_length(last_name) BETWEEN 2 AND 50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
