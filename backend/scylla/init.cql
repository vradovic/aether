CREATE KEYSPACE IF NOT EXISTS aether
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE aether;

CREATE TABLE IF NOT EXISTS messages (
    id timeuuid,
    conversation_id uuid,
    sender_id uuid,
    client_message_id uuid,
    body text,
    PRIMARY KEY (conversation_id, id)
) WITH CLUSTERING ORDER BY (id ASC);
