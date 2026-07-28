---
trigger: always_on
---

# System Architecture & Core Stack V1

This repository contains a high-throughput, eventually consistent real-time chat system. It strictly separates the hot-path Data Plane from the relational Control Plane.

*   **Language:** Go (1.18+)
*   **Message Broker:** NATS (Core for volatile fan-out, JetStream for durable queues)
*   **Databases:** 
    *   **ScyllaDB:** Immutable, masterless storage for chat messages.
    *   **PostgreSQL:** Relational CRUD for users, auth, and conversation metadata.
    *   **Redis:** In-memory state tracking for the Gateway Director API.
*   **Ingress:** Layer 7 load balancing is utilized exclusively for the API microservice. Gateways are accessed directly by clients via dynamic routing from the Director API.

---

## Service Boundaries

Agents must not bleed responsibilities between these three microservices.

### 1. Gateway Microservice (WebSockets)
*   **State:** Strictly stateless (not entirely). Uses memory only for WebSocket reference counting and Director API heartbeats.
*   **Data Plane (Chat):** Subscribes to Core NATS for fire-and-forget message fan-out.
*   **Control Plane (Kicks/Auth):** Subscribes to JetStream to guarantee delivery of system control events (like evictions) across all nodes. Do not use Queue Groups here; every gateway node must process every control event.
*   **Auth:** Clients provide a long-lived JWT for authentication upon connection.
*   **Subscriptions:** When a client connects, a database query is made to fetch the users conversations. Then the service subscribes to all of the conversations to NATS Core. The service does not create multiple subscriptions for the same conversation, instead it keeps a reference count for each subscription, so when a client disconnects, it subtracts his conversations from the counting.

### 2. Chat Microservice (Message Writers)
*   **Ingest:** Consumes incoming messages from JetStream using a Pull Consumer and a Queue Group so exactly one worker processes each message.
*   **Database:** Writes exclusively to ScyllaDB. Do not query PostgreSQL in this service.
*   **Durability:** The agent must only acknowledge the JetStream message after the ScyllaDB write returns successfully.

### 3. API Microservice (CRUD & Routing)
*   **Role:** Handles all REST traffic and manages PostgreSQL.
*   **Director API:** Implements the routing logic, querying Redis to direct new WebSocket clients to the gateway node with the lowest connection count.
*   **Events:** Publishes mandatory control events to JetStream.

---

## Database Paradigms

Agents must respect the masterless nature of ScyllaDB and avoid relational anti-patterns. (Refer to the repository's migration files for concrete table structures).

*   **No Auto-Increments:** Do not use SQL sequences or Lightweight Transactions (LWTs) for message IDs. 
*   **Primary Key Strategy:** Messages are partitioned by the conversation and clustered chronologically using Type 1 TimeUUIDs. 
*   **ID Generation:** The Go application must generate the `gocql.TimeUUID()` before insertion into ScyllaDB.

## Tooling

Something to keep in mind:
* Postgres queries and migrations are in the backend/sql directory. Goose is used for migrations and sqlc is used for generating go code from raw sql queries.