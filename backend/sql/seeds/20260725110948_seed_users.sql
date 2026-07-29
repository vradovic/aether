-- +goose Up
INSERT INTO users (id, username, email, password_hash, first_name, last_name)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'jdoe', 'john.doe@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'John', 'Doe'),
    ('00000000-0000-0000-0000-000000000002', 'jsmith', 'jane.smith@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'Jane', 'Smith'),
    ('00000000-0000-0000-0000-000000000003', 'mgarcia', 'maria.garcia@example.com', '$2a$12$eImiTXuWVxfM37uY4JANjO8e.G7Fid82n.u4R/E/3c8b4J3b4kC1K', 'Maria', 'Garcia');
