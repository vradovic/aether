INSERT INTO contacts (user1_id, user2_id)
VALUES
    -- John <-> Jane
    (
        LEAST(
                (SELECT id FROM users WHERE username = 'jdoe'),
                (SELECT id FROM users WHERE username = 'jsmith')
        ),
        GREATEST(
                (SELECT id FROM users WHERE username = 'jdoe'),
                (SELECT id FROM users WHERE username = 'jsmith')
        )
    ),
    -- John <-> Maria
    (
        LEAST(
                (SELECT id FROM users WHERE username = 'jdoe'),
                (SELECT id FROM users WHERE username = 'mgarcia')
        ),
        GREATEST(
                (SELECT id FROM users WHERE username = 'jdoe'),
                (SELECT id FROM users WHERE username = 'mgarcia')
        )
    ),
    -- Jane <-> Maria
    (
        LEAST(
                (SELECT id FROM users WHERE username = 'jsmith'),
                (SELECT id FROM users WHERE username = 'mgarcia')
        ),
        GREATEST(
                (SELECT id FROM users WHERE username = 'jsmith'),
                (SELECT id FROM users WHERE username = 'mgarcia')
        )
    );
