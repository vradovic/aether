-- +goose Up
INSERT INTO users (username, email, password_hash, first_name, last_name)
VALUES
    ('jdoe', 'john.doe@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'John', 'Doe'),
    ('jsmith', 'jane.smith@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'Jane', 'Smith'),
    ('mgarcia', 'maria.garcia@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'Maria', 'Garcia');

-- +goose Down
SELECT 'down SQL query';
