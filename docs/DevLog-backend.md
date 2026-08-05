# Backend Dev Log

## Live DB Testing

0. Prerequisites check

```sh
docker --version
docker compose version
go version
```

1. Review .env

Make sure the .env already has the right DB values.

2. Start the DB + Redis containers

Start only the data services first, not api/worker (they need migrations applied first).

```sh
docker compose -f docker-compose.dev.yml up -d db redis
```

Wait for them to be healthy.

```sh
docker compose -f docker-compose.dev.yml ps
```

Should see `jdz-db` and `jdz-redis` with healthy status.
The healthcheck does `pg_isready` / `redis-cli ping`.

Verify the DB is accepting connections.

```sh
docker exec jdz-db pg_isready -U postgres -d jingdezhen_ceramics_db
```

3. Apply migrations

Ensure `$(go env GOPATH)/bin` is on the path.

```sh
echo `export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
source ~/.zshrc
```

Install the migrate CLI on the host.

```sh 
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate -version
```

Then apply all migrations.

```sh
make migrate-up
```

4. Verify the schema

List all tables and check the migration version.

```sh
# Which migration version is applied?
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "SELECT version, dirty FROM schema_migrations;"
# Expect: version=10, dirty=f
# 
# List all tables (should be ~20 tables)
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "\dt"
```

Spot-check a few of the things the migrations built.

```sh 
# RBAC seed: 5 staff roles + 14 permissions + role_permissions wired
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "SELECT key FROM roles ORDER BY key;"
# Expect: content_editor, customer, customer_service, ecommerce_operator, super_admin, travel_planner
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "SELECT COUNT(*) FROM permissions;"
# Expect: 14
# The new deleted_at column from migration 000010
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "\d users" | grep deleted_at
# Expect: deleted_at | timestamp with time zone | nullable
```

5. Test rollback (optional, but recommeded once)

Verify the down migrations work.

```sh
make migrate-down
# then verify:
docker exec jdz-db psql -U postgres -d jingdezhen_ceramics_db -c "SELECT version FROM schema_migrations;"
# Expect: 9

# Re-apply
make migrate-up
# Expect: version=10, dirty=f
```

6. Seed a super_admin user (needed to exercise CMS endpoints)

The RBAC migration seeds roles/permissions but no users.
The CMS endpoints require a super_admin for publish actions or content_editor for write actions.
Create one manually.

```sh
docker exec -it jdz-db psql -U postgres -d jingdezhen_ceramics_db
```

Inside psql, run:

```sql 
-- 1. Create an active user (password hash below = bcrypt of "password123")
INSERT INTO users (id, nickname, email, password_hash, is_active, auth_provider)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'Super Admin',
  'admin@jingdezhen.test',
  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
  TRUE,
  'email'
);
-- 2. Assign super_admin role
INSERT INTO user_roles (user_id, role_id)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  (SELECT id FROM roles WHERE key = 'super_admin')
);
```

A super_admin must enroll 2FA before they can get a full session JWT.
(This is hard-enforced at login: you will get `Err2FAEnrollmentRequired` with a 15-min pending token.)
The login flow is:
`POST /auth/login` -> 412 with pending token -> `POST /auth/2fa/pending-enroll` (returns QR) -> `POST /auth/2fa/pending-confirm` with TOTP code -> real JWT.
To skip 2FA testing, use a content_editor instead.

```sql
INSERT INTO user_roles (user_id, role_id)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  (SELECT id FROM roles WHERE key = 'content_editor')
);
```

Exit psql with `\q`.

7. Run the API and smoke-test

```sh
make up
make logs-api
```

If you see `Successfully connected to the database!` + `Successfully connected to Redis!`, the stack is up.

Smoke-test a few endpoints.

```sh
# Public health check
curl -s http://localhost:1323/ | head

# Public content read (ceramic stories — empty, but should return 200 + [])
curl -s "http://localhost:1323/ceramicstory?locale=en-US"

# Login as the content_editor you seeded
curl -s -X POST http://localhost:1323/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@jingdezhen.test","password":"password123"}'
# → should return an access_token (content_editor has no 2FA mandate)
```

8. Tear down / reset

Wipe everything and start fresh (drops the DB volume + Redis volume).

```sh
make down        # stops containers and deletes volumes
make up          # recreate
make migrate-up  # re-apply migrations
```

**Quick reference**

 ┌─────────────────────────────────────────────────┬────────────────────────────────┐ 
 │ Command                                         │ What it does                   │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ docker compose -f docker-compose.dev.yml up -d  │ Start only Postgres + Redis    │ 
 │ db redis                                        │                                │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ make migrate-up                                 │ Apply all pending migrations   │ 
 │                                                 │ (needs migrate CLI)            │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ make migrate-down                               │ Roll back the last migration   │ 
 │                                                 │ only                           │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ docker exec jdz-db psql -U postgres -d          │ Open a psql shell in the DB    │ 
 │ jingdezhen_ceramics_db                          │ container                      │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ docker exec jdz-db pg_isready -U postgres -d    │ Check DB readiness             │ 
 │ jingdezhen_ceramics_db                          │                                │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ make up / make down / make stop                 │ Full stack lifecycle           │ 
 ├─────────────────────────────────────────────────┼────────────────────────────────┤ 
 │ make logs-api / make logs-worker                │ Tail service logs              │ 
 └─────────────────────────────────────────────────┴────────────────────────────────┘

## API endpoints testing via curl

Keeps `make logs-api` or `docker compose -f docker-compose.dev.yml logs -f api` running in terminal 1 for logs.
Run all curl commands in terminal 2.

### Line continuation

 Use \ at the end of each line to break a command across lines. The \ must be the
 last character (no trailing spaces). Bash shows a > prompt on the next line. Press
 Enter on the last line (no \) to execute.

### Common curl flags

 ┌───────────────────────────────────┬──────────────────────────────────────────────┐ 
 │ Flag                              │ Meaning                                      │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -s                                │ Silent (no progress meter)                   │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -i                                │ Include response headers (shows status code  │ 
 │                                   │ + body)                                      │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -X POST / -X PUT / -X DELETE      │ HTTP method (GET is default)                 │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -H "Content-Type:                 │ Request header                               │ 
 │ application/json"                 │                                              │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -d '{"key":"value"}'              │ Request body                                 │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -o /dev/null -w "%{http_code}\n"  │ Print only the status code                   │ 
 ├───────────────────────────────────┼──────────────────────────────────────────────┤ 
 │ -d @/tmp/body.json                │ Read body from a file (avoids quoting        │ 
 │                                   │ issues)                                      │ 
 └───────────────────────────────────┴──────────────────────────────────────────────┘ 

### Response status codes

 ┌──────┬──────────────┬────────────────────────────────────────────────────┐
 │ Code │ Meaning      │ Action                                             │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 200  │ OK           │ Success                                            │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 201  │ Created      │ Success (POST that creates a resource)             │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 204  │ No Content   │ Success (DELETE)                                   │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 400  │ Bad Request  │ Invalid body / validation failed — check your JSON │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 401  │ Unauthorized │ No token / invalid token — get a new JWT           │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 403  │ Forbidden    │ Authenticated but lacks permission (wrong role)    │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 404  │ Not Found    │ Wrong URL or resource doesn't exist                │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 409  │ Conflict     │ Duplicate / workflow state conflict                │
 ├──────┼──────────────┼────────────────────────────────────────────────────┤
 │ 500  │ Server Error │ Bug — check logs in terminal 1                     │
 └──────┴──────────────┴────────────────────────────────────────────────────┘

### Step 1 — Login (get a JWT)

```sh                                                                          
curl -s -X POST http://localhost:1323/auth/login \                              
  -H "Content-Type: application/json" \                                         
  -d '{"email":"admin@jingdezhen.test","password":"password123"}'               
```                                                                            

Copy the access_token value (the long eyJ... string). Store it in a shell variable 
to reuse:

```sh                                                                        
TOKEN="eyJ...paste-token-here..."                                            
```

Verify it's set: `echo $TOKEN` should print the token.

### Step 2 — Use the token in protected requests

Add `-H "Authorization: Bearer $TOKEN"` to any request that requires auth:

```sh                                                                           
# Get profile                                                                   
curl -s http://localhost:1323/profile \                                         
  -H "Authorization: Bearer $TOKEN"                                             
                                                                                
# List shipping addresses                                                       
curl -s http://localhost:1323/profile/addresses \                               
  -H "Authorization: Bearer $TOKEN"                                             
                                                                                
# Create an address                                                             
curl -s -X POST http://localhost:1323/profile/addresses \                       
  -H "Authorization: Bearer $TOKEN" \                                           
  -H "Content-Type: application/json" \                                         
  -d '{"recipient":"John","line1":"123 Main                                     
 St","city":"Jingdezhen","country":"CN","is_default":true}'                     
                                                                                
# GDPR data export                                                              
curl -s http://localhost:1323/profile/export \                                  
  -H "Authorization: Bearer $TOKEN"                                             
                                                                                
# Delete account (irreversible! skips for testing unless you mean it)           
# curl -s -X POST http://localhost:1323/privacy/delete-account \                
#   -H "Authorization: Bearer $TOKEN" \                                         
#   -H "Content-Type: application/json" \                                       
#   -d '{"confirm":"DELETE"}'                                                   
```                                                                            

### Step 3 — Public endpoints (no token needed)

 ```sh                                                                          
# Health check                                                                  
curl -s http://localhost:1323/                                                  
                                                                                
# Public content (ceramic stories, empty for now)                               
curl -s "http://localhost:1323/ceramicstory?locale=en-US"                       
                                                                                
# Public activities (Destinations & Local Lifestyle)                            
curl -s "http://localhost:1323/activities?locale=en-US"                         
```                                                                             

### Step 4 — Admin CMS endpoints (need content_editor role)

Your `admin@jingdezhen.test` user has content_editor — can create/edit/submit, but NOT 
approve/reject/unpublish (those need `super_admin`):

```sh                                                                           
# Create a ceramic story (draft)                                                
curl -s -X POST http://localhost:1323/admin/ceramicstory \                      
  -H "Authorization: Bearer $TOKEN" \                                           
  -H "Content-Type: application/json" \                                         
  -d '{"locale":"en-US","dynasty_name":"Ming Dynasty","slug":"ming-dynasty",
  "description":"The Ming dynasty porcelain era.","start_year":1368,
  "end_year":1644,"display_order":1}'                          
                                                                                
# List all stories (all statuses, admin view)                                   
curl -s "http://localhost:1323/admin/ceramicstory?locale=en-US" \               
  -H "Authorization: Bearer $TOKEN"                                             
                                                                                
# Submit for review (draft → in_review)                                         
curl -s -X POST http://localhost:1323/admin/ceramicstory/1/submit \             
  -H "Authorization: Bearer $TOKEN" \                                           
  -H "Content-Type: application/json" \                                         
  -d '{"locale":"en-US"}'                                                       
                                                                                
# Approve (in_review → published) — will return 403 (needs super_admin)         
curl -s -X POST http://localhost:1323/admin/ceramicstory/1/approve \            
  -H "Authorization: Bearer $TOKEN" \                                           
  -H "Content-Type: application/json" \                                         
  -d '{"locale":"en-US"}'                                                       
```                                                                            

### Step 5 — Pretty-print JSON

Pipe through `python3 -m json.tool` or `jq` for readable output:

```sh                                                                          
  curl -s http://localhost:1323/profile \                                      
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool                     
```                                                                            

### Quick reference — the full pattern

```sh                                                                           
# 1. Login → copy access_token                                                  
curl -s -X POST http://localhost:1323/auth/login \                              
  -H "Content-Type: application/json" \                                         
  -d '{"email":"admin@jingdezhen.test","password":"password123"}'               
                                                                                
# 2. Store token                                                                
TOKEN="eyJ..."                                                                  
                                                                                
# 3. Make authenticated requests                                                
curl -s http://localhost:1323/profile \                                         
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool                      
```                                                                            

### Troubleshooting

 ┌────────────────────────┬────────────────────┬────────────────────────────────────┐ 
 │ Symptom                │ Cause              │ Fix                                │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 000 status             │ API not reachable  │ Check docker ps + logs             │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 400 Invalid request    │ Body didn't parse  │ Check for curly quotes; use -d     │ 
 │ body                   │                    │ @file.json                         │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 401 Invalid            │ Wrong              │ Regenerate bcrypt hash, update DB  │ 
 │ credentials            │ email/password     │                                    │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 401 on protected route │ No/invalid token   │ Re-login, set $TOKEN               │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 403                    │ Wrong role         │ Check user_roles in psql           │ 
 ├────────────────────────┼────────────────────┼────────────────────────────────────┤ 
 │ 500                    │ Server bug         │ Check logs in terminal 1           │ 
 └────────────────────────┴────────────────────┴────────────────────────────────────┘

## `media_assets` table vs. scattered URLs

Scattered URL strings: Every entity just has a text column: `products.thumbnail_url`, `artist.avatar_url`, etc. The DB stores a full string.

What do `media_assets` and ordered galleries bring?

### Multiple images per entity

A product can have more than one `thumbnail_url`, e.g. a main short + 3 detail shots + a firing video.
If you go with N image columns or a comma-separated string, the ordering, captions and queries will be broken.

### Metadata the frontend needs

`media_assets` carries width, height, mime, duration. A bare URL string has none of it.
- `<img width height>` prevents layout shift (CLS), which is a core web vitals and SEO ranking signal. With a bare URL the frontend can't render dimensions until the image loads.
- Aspect-ratio CSS for placeholders, responsive `srcset`, video-duration badges, all need metadata the DB doesn't hold.

### CDN URL resolution in one place

With scattered strings, the URL is baked ino every row of every table. If you change OSS bucket, CDN domain, or want to add OSS image-processing params (`?x-oss-process=image/resize,w_400`), you do a data migration across products + artists + stories + activities. With `media_assets`, the DB stores the stable `oss_key`. The storage adapter resolves `oss_key` to CDN URL at read time. Config change = one adapter edit without data migration.

### Referential integrity

A FK means a product's gallery can't point at a deleted/renamed OSS object without surfacing it. A bare string can silently dangle.

### Reuse without duplication

One detail shot shared across a product, a ceramic story, and the artist's profile is one `media_assets` row referenced by 3 gallery rows.
With strings you paste the URL 3x and fix 3 places if the file moves.

### The video pipeline can't exist without it

Video streaming needs FFmpeg -> HLS transcoding, with the result stored in `media_assets.hls_key`. A bare `image_url` string can't carry it.

### Upload accounting + cost control

- `uploaded_by`, `created_at`
- orphan detection: assets referenced by nothing -> GC from OSS to save storage cost
- quota per admin

### Type-aware rendering

`kind image|video` lets the frontend pick `<img>` vs a player without sniffing the file extension out of a URL.

### Trade-off

More joins, more code, more migration surface.
Advantages are as above.

## Testcontainers-go harness

### Step 1 - Add the dependencies

```sh
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
go get github.com/testcontainers/testcontainers-go/modules/redis@latest
go mod tidy
```

### Step 2 - Create `internal/testutil/testdb.go`

This is the core. One helper spins up Postgres, applies all migrations, and returns a ready `*pgxpool.Pool`.
Two design choices:
- Apply migrations via `golang-migrate` embedded in the binary, not by shelling out to the `migrate` CLI. Because tests must run anywhere (`go test` and CI) without the `migrate` binary installed.
- Each test gets an isolated database on the same container  (create db -> migrate -> drop db on teardown), OR each test gets a fresh container. Fresh-container-per-test is slow (~3-5s each), isolated-db-per-test on a shared container is fast (~50ms each) and is the standard pattern. Here use a `sync.Once` lazy singleton container + a `WithTestDB(t)` helper that create db + migrates + returns a pool + registers `t.Cleanup` to drop the db.

### Step 3 - Embed the migrations into `testutil`

`go:embed` paths are relative to the embedding `.go` file.
`internal/testutil/` is a sibling of `internal/migrations/`, so `//go:embed ../migrations/*.up.sql` won't compile, because embed can't escape the module dir upward.
2 clean options:
- move/copy migrations under testutil's embed root: Add a `Makefile/go:generate` step that copies `internal/migrations/*.sql` -> `internal/testutil/migrations/` before tests run, OR symlink. Simplest robust version: a tiny `gen.go` with `//go:generate cp -r ../../migrations ./migrations` and run `go generate ./internal/testutil` in CI. 
- (cleaner, no copy) create `internal/migrations/embed.go` in the migrations package itself with the `//go:embed *.up.sql *.down.sql`, exporting `migrations.FS`. Then `testutil` imports `internal/migrations` and passes `migrations.FS` to `iofs.New`. This keeps a single source of truth and needs no generate step.

### Step 4 - Add the Redis helper

`internal/testutil/testredis.go`
No migrations, just a ping.
Use a fresh DB index per test (or flush on cleanup) so tests stay isolated.

### Step 5 - Add a Makefile target + a `.env.test`

```makefile
## test-integration: run integration tests (requires Docker for testcontainers)    
test-integration:                                                           go test -tags=integration ./...
```

Tests that hit real PG/Redis should live in `*_test.go` files that call `testutil.NewDBPool(t)` / `testutil.NewRedisClient(t)`.
No build tag needed: testcontainers is a normal dep. The `Docker available?` check happens at `t.Fatalf` time.

Add to `.env.example`:
```
# Integration tests auto-provision Postgres + Redis via testcontainers-go.
# No DATABASE_URL needed for `go test`. TESTCONTAINERS_RYUK_DISABLED=true
# disables the reaper (use only in CI where the runner is ephemeral).
```

### Step 6 - verify the harness itself with a smoke test

Write `internal/testutil/testutil_test.go` to prove the harness works before any module uses it.

Run:
```sh
go test -run TestNew ./internal/testutil/... -v
```

First run pulls the imgaes (~30s). Subsequent runs reuse the container via Ryuk and finish in < 1s.

### Step 7 - First real integration test

Create test files for every service, e.g. `internal/modules/order/order_service_test.go`.
After the failing checkout, query `SELECT COUNT(*) FROM orders` and `SELECT stock FROM skus`, both must be unchanged.

### Step 8 - CI integration

```yaml
# .github/workflows/ci.yml (test job)
- uses: actions/setup-go@v5
  with: { go-version: '1.24' }
- run: cd backend && go mod download
- run: cd backend && go vet ./...
- run: cd backend && go test -race -cover ./...
  env:
    TESTCONTAINERS_RYUK_DISABLED: "true" # GH runner is ephemeral; ryuk unnecessary
```

### What is a `*pgxpool.Pool`?

It's a connection pool - a small, long-lived bag of TCP connections to PostgreSQL that many goroutines share.
Without a pool, every time a repository wants to run a query it would open a fresh TCP connection (dial -> TLS handshake -> auth -> query -> close). That's ~5-20ms of overhead per query and it hammers Postgres' connection limit. A pool keeps N connections warn (default ~4, tunable) and hands them out on demand.

```
repository.QueryRow() -> pool.Get() -> [reuses a warm conn] -> pool.Put()
```

`pgxpool.Pool` is the single thing every repository takes as a constructor argument.

```go
func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}
```

A `*pgxpool.Pool` is not a transaction. A transaction is `pool.BeginTx()` which returns a `pgx.Tx` that's a single connection held for the duration of the tx, then returned to the pool on Commit/Rollback.
The order repo's `CreateOrder` does exactly that: it borrows one connection from the pool for the whole insert + decrement + commit, returns it, and the next query can reuse it.

### Why is each test isolated?

Because shared state between tests is how test suites rot.
3 concrete failure modes isolation prevents:
- order dependence. Test A inserts a user with id=1 and asserts `COUNT(*) = 1`. Test B inserts another and seerts `COUNT(*)=2`. Run B before A and both break for reasons unrelated to what either is testing. With per-test DBs, both see `COUNT(*)=1` regardless of run order.
- leftover data from a previous test's failure. Test A inserts an order, crashes mid-assert.Test N queries "all orders" and finds A's orphan row. Now B is debugging A's bug. With isolation, A's DB is dropped on cleanup and B starts clean.
- cross-contamination of invariants. The order/stock test asserts `stock=0` after buying 2. If it ran against a DB where another test already decremented that SKU, the assertion is meaningless. Each test seeds its own SKU, so "stock 2" is a fact the test owns, not an assumption about prior state.

Here, "each test" = each `t` that calls `NewDBPool(t)`.
A service test file with 8 test functions creates + destroys 8 ephemeral DBs over its run, all sharing one container.

`NewDBPool(t)` hands a fresh, fully-migrated, throwaway database, scoped to exactly one test, that goes away when the test ends.

## What's the difference between `order_service_test.go` and `payment_service_test.go`?

`payment_service_test.go` uses a `fakeRepo`.

| | `order_service_test.go` | `payment_service_test.go` |
| --- | --- | --- |
| Layer | Integration (real PG via testcontainers) | Unit (mocked repo) |
| What's under test | `order.Repository.CreateOrder`: the SQL tx + atomic stock decrement | `payment.Service.HandleWebhook`: the orchestration logic |
| Repo used | `order.NewRepository(pool)`: the real repo, againist a real DB | `fakeRepo`: an in-memory stand-in |
| Docker needed | Yes | No |
| What a bug looks like | Wrong SQL, schema drift, tx not rolling back | Wrong control flow: double-enqueue, accepting an unsigned webhook |

The payment service's job in `HandleWebhook` is orchestration: verify the signature -> upsert the event -> if new, enqueue a finalize job.
The interesting bugs are:
- a replayed webhook enqueues the finalize job twice (double-charge risk)
- an unsigned webhook gets accepted (security boundary)
- an unknown gateway name is rejected
None of those depend on real SQL. The idempotency is the control flow - if `UpsertWebhook` says 'already existed', don't enqueue again - and that control flow is the same whether the upsert hits real Postgres or an in-memory map.

repository_test: user, address, product

## GitHub Actions CI pipeline

It's 3 layers.

### `.github/workflows/*.yml` - the pipeline itself

This is the only strictly required file. Everything else is config consumed by this workflow or by tools it runs.
A workflow defines:
- triggers (`on:`): when it runs (push, PR, schedule, manual)
- jobs: parallel / sequential units of work, each on a runner
- steps: checkout, setup tools, run commands, use actions

You can have multiple workflow files, e.g. `ci.yml`, `deploy.yml`, `release.yml`. They are independent pipelines.
This project's workflow is just `ci.yml` with 5 jobs: lint -> unit -> integration -> build -> security.

### Tool config files - consumed by steps inside the workflow

These are not GitHub-specific. The workflow just invokes the tools, which read their own configs.

 ┌────────────────────────────────────┬──────────────────────┬──────────────────────┐ 
 │ File                               │ Tool                 │ What it configures   │ 
 ├────────────────────────────────────┼──────────────────────┼──────────────────────┤ 
 │ backend/.golangci.yml              │ golangci-lint        │ which linters, what  │ 
 │                                    │                      │ to exclude           │ 
 ├────────────────────────────────────┼──────────────────────┼──────────────────────┤ 
 │ backend/.golangci-lint-version /   │ golangci-lint-action │ optional: pin the    │ 
 │ .tool-versions                     │                      │ linter version via   │ 
 │                                    │                      │ file instead of      │ 
 │                                    │                      │ inline               │ 
 ├────────────────────────────────────┼──────────────────────┼──────────────────────┤ 
 │ .editorconfig / prettier config    │ any formatter        │ style rules          │ 
 └────────────────────────────────────┴──────────────────────┴──────────────────────┘ 

 ### `.github/dependeabot.yml` - automated dependency maintenance

 This is not part of the CI pipeline, it's a separate GitHub feature that opens PRs to bump outdated deps.
 It doesn't run tests; it just triggers the CI pipeline so the pipeline validates the bumps.
