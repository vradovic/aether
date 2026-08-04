# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository shape

Aether is a real-time chat system. All tracked code lives in `backend/` — a single Go module (`github.com/vradovic/aether/backend`) that builds four binaries from `cmd/`. There is no client in the repo (the Flutter client was deleted in `d0f153c`).

Root-level `deployments/`, `scylla/`, and `sql/` are stale duplicates of the directories under `backend/`. Edit the ones under `backend/`; `docker-compose.yml` resolves almost everything relative to `backend/` (the one exception is the `scylla` service's `../scylla:/scylla:ro` mount, which points at the root copy and is unused).

`.agents/rules/backend-v1.md` is an always-on architecture rules file — read it, but see "Where the code diverges from the rules doc" below before treating it as a description of what exists.

## Commands

All Go commands run from `backend/`.

```bash
go build ./...
```

```bash
go vet ./...
```

`go vet` currently fails on `cmd/issuetoken/main.go:21` (a `%s` directive passed to `Fprintln`). Pre-existing, unrelated to most changes.

Run the full test suite (requires a running Docker daemon — see Testing):

```bash
go test ./... -count=1
```

Run one package, or one test:

```bash
go test ./internal/api/conversations/ -run TestGetMessages -count=1 -v
```

Bring up the whole stack (Postgres, NATS + JetStream, ScyllaDB, all three services, nginx, Prometheus):

```bash
docker compose up --build
```

`backend/.env` (gitignored) supplies `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, and `JWT_SIGNING_KEY`. The signing key must be at least 32 bytes or both services refuse to start.

Scale the WebSocket tier. nginx resolves upstream addresses once at startup, so new replicas get no traffic until it reloads:

```bash
docker compose up -d --scale realtime-service=5 && docker compose exec nginx nginx -s reload
```

Mint a JWT for manual testing or for `tests/k6/tokens.json`:

```bash
go run ./cmd/issuetoken <userID> <signingKey>
```

Regenerate `internal/db/` after touching `sql/migrations/` or `sql/queries/`:

```bash
sqlc generate
```

Load test the WebSocket path (`URLS` is a comma-separated list of `ws://…/ws` endpoints):

```bash
k6 run tests/k6/script.js -e URLS=ws://localhost:8000/ws
```

## Service topology

Three long-running services plus nginx, all defined in `backend/docker-compose.yml`:

| Service | Entry point | Port | Owns |
| --- | --- | --- | --- |
| `api-service` | `cmd/api` | 8081 | REST CRUD, Postgres |
| `realtime-service` | `cmd/realtime` | 8080 | WebSockets, NATS Core fan-out |
| `worker-service` | `cmd/worker` | 8079 (metrics) | JetStream consumer, ScyllaDB writes |

`nginx` is the single ingress on host port 8000 (`deployments/nginx.conf`): `/api/*` is prefix-stripped and proxied to `api-service`, `/ws` goes to `realtime-service` with `least_conn` balancing because WebSocket connections are long-lived and unevenly distributed.

Prometheus scrapes `realtime-service` via Docker service discovery (so it picks up scaled replicas automatically) and `worker-service` statically.

## Message flow

This is the core of the system and spans all three services:

1. A client opens `/ws?token=<jwt>`. `realtime.ServeWs` validates the JWT, queries Postgres for that user's conversations, and registers the connection with the `Router`.
2. `client.readPump` publishes each inbound message to **NATS JetStream** on `messages.unprocessed.<conversationID>` (stream `MESSAGES`, workqueue retention).
3. `worker.Process` pulls from the durable consumer `message-service`, assigns a `gocql.TimeUUID()`, writes to ScyllaDB, and only then acks. Write failures `Nak`; unparseable payloads `Term`.
4. The worker republishes the persisted message to **NATS Core** on `messages.processed.<conversationID>`.
5. `realtime.Router` fans it out to every connected client subscribed to that conversation.

Two NATS usages, deliberately different: JetStream (durable, queue-grouped, exactly one worker per message) for ingest, Core NATS (fire-and-forget) for fan-out.

### Router subscription model

`internal/realtime/router.go` is the trickiest file. The `Router` goroutine owns `register`/`unregister` and serializes all registry mutation. Per conversation it keeps one NATS subscription with a **reference count**, so N clients in the same conversation share one subscription; the subscription is torn down when the count hits zero. Each subscription gets its own `processConversation` goroutine that exclusively owns the channel and exits when the channel closes on unsubscribe.

Slow clients are dropped, not blocked: `deliver` does a non-blocking send and unregisters any client whose 64-message buffer is full.

## Package layout and conventions

- `internal/api/` — one subpackage per domain (`auth`, `contacts`, `conversations`), each with a `Handler` / service pair. Handlers depend on a `ServiceInterface` and services on a `Querier` interface, both declared in the consuming package, which is what makes the table-driven fake-based tests possible.
- `internal/api/httputil/` — JSON decode/encode and canned error responses. `DecodeJSON` caps bodies at 1 MiB, rejects unknown fields, and rejects trailing JSON values.
- `internal/core/` — JWT issuing/parsing and UUID helpers, shared by the API and realtime services.
- `internal/db/` — **generated by sqlc; never edit by hand.**
- Routes are registered on `http.ServeMux` with Go 1.22 method+wildcard patterns (`"PATCH /conversations/{conversationID}"`). Auth is `api.Middleware.RequireAuth`, which unwraps the bearer token and passes `userID string` as a third argument to the handler.
- The realtime service authenticates via a `?token=` query parameter instead, since browsers can't set headers on a WebSocket handshake.

## Testing

Tests use external test packages (`package conversations_test`) to exercise only the public surface — this is a standing rule in `.agents/rules/backend-v1.md`.

Handler tests are pure unit tests over hand-written fakes. Service tests call `apitest.StartDatabase`, which spins up a real `postgres:18-alpine` container via testcontainers and runs `sql/migrations` through goose, so **`go test ./...` needs Docker running** and takes ~10s per service package. There is no `-short` guard, so `-short` will not skip them.

## Database paradigms

Postgres (via sqlc + goose) is the relational control plane: users, contacts, conversations, participants. ScyllaDB is the message store, partitioned by `conversation_id` and clustered by a Type-1 TimeUUID.

Both the API and worker services connect to Scylla (`SCYLLA_HOSTS`, `SCYLLA_KEYSPACE`) via `core.NewScyllaCluster`. Message history pages forward on that clustering key: `GET /conversations/{id}/messages?after_id=<timeuuid>` returns up to 100 messages oldest first, and the client advances `after_id` to the last ID it received. Omitting `after_id` starts from the oldest message; a non-Type-1 UUID is a 400.

Do not introduce SQL sequences or Scylla lightweight transactions for message IDs. The Go application generates `gocql.TimeUUID()` before insertion.

## Where the code diverges from the rules doc

`.agents/rules/backend-v1.md` describes a design that is partly not built yet. Verify before relying on it:

- **No Redis and no "Director API."** Nothing in `go.mod` or compose references Redis. Gateway selection is done by nginx `least_conn`, not by clients querying a director for the lowest-connection node.
- **No JetStream control plane.** The realtime service subscribes only to the Core NATS data plane; there are no eviction/kick control events.
- **Message history reads ScyllaDB, not Postgres.** `conversations.GetMessages` authorizes against Postgres (`IsConversationParticipant`) and then reads the same ScyllaDB table the worker writes to, through `conversations.ScyllaReader`. The Postgres `messages` table still exists in `sql/migrations` but has no queries and no writers.
