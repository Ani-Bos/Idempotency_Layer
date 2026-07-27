# Idempotency-Key Middleware for HTTP APIs

## Decisions

### 1. Store interface: `Start/Complete` instead of `Lock/Unlock/Set/Get`

**Decision:** A single `Start` call atomically checks the cache, detects
in-progress state, or reserves a new slot. `Complete` stores the result and
releases the mutex i.e releasing the lock

**Rejected:** Separate `Lock`, `Unlock`, `Set`, `Get` methods.
- *Why rejected:* A race window exists between `Get` (miss) and `Lock` (acquire).
  Two concurrent requests can both see a miss and double-execute the handler.
  `Start` makes check-and-reserve atomic.

### 2. In-Preogress retries block on a channel, not 409

**Decision:** When a retry arrives while the first execution is still running,
the retry blocks and we are using a Go channel to do it. When the executor finishes, the result is
broadcasted to all waiters, who replay it identically.

**Rejected:** Return `409 Conflict` for in-progress duplicates.
- *Why rejected:* The caller is retrying because they did not receive a response
  (network timeout, etc.). A 409 tells them their retry is invalid, which
  violates the core promise of idempotency: retries must be safe and invisible.

### 3. Request fingerprint includes method, path, query and body instead of just idempotency key

**Decision:** The idempotency key alone is not enough. We compute a SHA-256
fingerprint of `method + path + query + body`. If the same key is reused with a
different payload, the middleware returns `409 Conflict`.

**Rejected:** Trust the key alone.
- *Why rejected:* A reused key with a different body is almost certainly a client
  bug. Silently replaying the old response would be silent data corruption.
  Failing loud with 409 is safer.

### 4. Cache successes and 4xx, but NOT 5xx or panics

**Decision:** Responses with status `< 500` are cached. Responses `>= 500` and
handler panics are treated as *transient* — the entry is removed so retries
re-execute.

**Rejected:** Cache every response indiscriminately.
- *Why rejected:* A transient 500 or timeout would "poison" the key forever.
  The first failure becomes permanent for that key. Transient failures should be retryable at any cost

### 5. Requests without a key pass through untouched
**Decision:** If no `Idempotency-Key` header is present, the middleware is
transparent. The handler runs normally every time.

**Rejected:** Auto-generate a key from the request fingerprint.
- *Why rejected:* The caller would not know the auto-generated key, so retries
  could not send it. This creates a false promise of safety.

### 6. In-memory LRU + TTL for bounded memory

**Decision:** `MemoryStore` uses a `map` + `container/list` LRU. Each entry has
a TTL. A background lazy eviction removes expired entries on access, and evicts
the oldest entries when max size is exceeded.

**Rejected:** Unbounded `map` with no eviction.
- *Why rejected:* A malicious or buggy client sending unique keys would OOM the
  server.

---
