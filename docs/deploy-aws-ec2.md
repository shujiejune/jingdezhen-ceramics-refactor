# Deploying the Jingdezhen Ceramics Platform to AWS EC2

A step-by-step guide for deploying the full stack (Go API, worker, PostgreSQL, Redis, frontend SSR, Caddy, Prometheus, Grafana) to a single AWS EC2 instance using Docker Compose.

---

## Prerequisites

- An **AWS account** with permission to create EC2 instances, security groups, and elastic IPs.
- A **domain name** pointed at the EC2 instance's elastic IP (for TLS via Caddy / Let's Encrypt).
- The repo cloned locally, with `backend/.env.prod` ready (copy from `backend/.env.prod.example`).

---

## Step 1: Choose an EC2 Instance Type

The app runs as a modular monolith on a single VPS. For the MVP, a **`t3.medium`** (2 vCPUs, 4 GB RAM) is the minimum viable size. For production with headroom, use **`t3.large`** (2 vCPUs, 8 GB RAM) or **`t3.xlarge`** (4 vCPUs, 16 GB RAM).

| Instance | vCPUs | RAM | Cost (us-east-1, on-demand) | Suitability |
|----------|-------|-----|----------------------------|-------------|
| `t3.small` | 2 | 2 GB | ~$15/mo | Too small — PostgreSQL + Redis + API + SSR + monitoring will OOM |
| `t3.medium` | 2 | 4 GB | ~$30/mo | Minimum viable — tight but works for low traffic |
| `t3.large` | 2 | 8 GB | ~$60/mo | **Recommended** — comfortable headroom for all services |
| `t3.xlarge` | 4 | 16 GB | ~$120/mo | High-traffic production with monitoring retention |

**Yes, AWS EC2 is suitable for this project.** The app is designed as a single-VPS modular monolith (TDD §2.1) — Docker Compose on one instance is the intended deployment topology. The original design targeted a Hong Kong VPS, but AWS EC2 in any region works identically; just pick a region close to your users (e.g. `us-east-1` for North America, `ap-southeast-1` for Asia).

---

## Step 2: Launch the EC2 Instance

### 2.1 Create the instance

1. **AWS Console** → EC2 → **Launch Instance**.
2. **Name:** `jingdezhen-prod` (or whatever you prefer).
3. **AMI:** Ubuntu Server 24.04 LTS (x86, `ami-...`).
4. **Instance type:** `t3.large` (or `t3.medium` for MVP).
5. **Key pair:** Create or select an existing SSH key pair (`.pem`).
6. **Storage:** 30 GB gp3 SSD (minimum — increase if storing media locally).
7. **Security group:** Create a new one (see §2.2 below).
8. **Launch.**

### 2.2 Security group inbound rules

| Port | Protocol | Source | Purpose |
|------|----------|--------|---------|
| 22 | TCP | Your IP only | SSH access |
| 80 | TCP | `0.0.0.0/0` | HTTP (Caddy redirects to HTTPS) |
| 443 | TCP | `0.0.0.0/0` | HTTPS (Caddy TLS) |

**Do NOT expose:** 1323 (Fiber API), 5432 (PostgreSQL), 6379 (Redis), 9090 (Prometheus), 3001 (Grafana), 3000 (SSR). These are internal to the Docker network and accessed only via Caddy or SSH tunneling.

### 2.3 Allocate an Elastic IP

1. **AWS Console** → EC2 → **Elastic IPs** → **Allocate**.
2. **Associate** the elastic IP to your `jingdezhen-prod` instance.
3. Note the IP (e.g. `203.0.113.42`) — you'll point your domain's A record at this.

---

## Step 3: Configure DNS

1. In your DNS provider (Route 53, Cloudflare, Namecheap, etc.), create:
   - **A record:** `your-domain.com` → `203.0.113.42` (your elastic IP).
   - **A record (optional):** `www.your-domain.com` → `203.0.113.42`.
2. Wait for DNS propagation (check with `dig your-domain.com`).

Caddy will automatically provision TLS certificates via Let's Encrypt when it starts and sees the domain resolving to the instance.

---

## Step 4: Install Docker on the EC2 Instance

SSH into the instance:

```bash
ssh -i ~/.ssh/your-key.pem ubuntu@203.0.113.42
```

Install Docker + Docker Compose:

```bash
# Update packages
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sudo sh

# Add ubuntu user to docker group (so you don't need sudo)
sudo usermod -aG docker ubuntu

# Install docker-compose plugin (v2)
sudo apt install docker-compose-plugin -y

# Verify
docker --version
docker compose version
```

Log out and back in for the docker group to take effect.

---

## Step 5: Clone the Repo and Configure Environment

```bash
# Clone
git clone https://github.com/shujiejune/jingdezhen-ceramics-refactor.git
cd jingdezhen-ceramics-refactor

# Create the production env file
cd backend
cp .env.prod.example .env.prod
```

Edit `.env.prod` with your real values:

```bash
nano .env.prod
```

Key variables to fill in:

```ini
# --- Database ---
DB_USER=jingdezhen
DB_PASSWORD=<generate a strong random password>
DB_NAME=jingdezhen_ceramics_db
DB_PORT=5432
DB_SSLMODE=disable

# --- Auth ---
JWT_SECRET=<generate a 64-char random string>
CONSENT_HMAC_KEY=<generate a 32-char random string>
TWO_FA_ENCRYPTION_KEY=<generate a 32-char hex string>

# --- Server ---
SERVER_PORT=1323
CLIENT_ORIGIN=https://your-domain.com
SITE_BASE_URL=https://your-domain.com
ADMIN_EMAIL=admin@your-domain.com

# --- Deployment ---
SITE_DOMAIN=your-domain.com
RATE_LIMIT_MAX=100

# --- External services (fill in real keys for production) ---
BREVO_API_KEY=<your Brevo key>
BREVO_SENDER_EMAIL=noreply@your-domain.com
BREVO_SENDER_NAME=Jingdezhen Ceramics

# Payments (sandbox for MVP; switch to live post-onboarding)
AIRWALLEX_ENV=sandbox
AIRWALLEX_CLIENT_ID=<sandbox client ID>
AIRWALLEX_API_KEY=<sandbox API key>
PAYPAL_ENV=sandbox
PAYPAL_CLIENT_ID=<sandbox client ID>
PAYPAL_CLIENT_SECRET=<sandbox secret>

# Storage (Alibaba Cloud OSS or leave local for MVP)
STORAGE_MODE=local
STORAGE_LOCAL_DIR=./_media
STORAGE_PUBLIC_BASE_URL=/media

# Monitoring
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=<strong password>
```

Generate random secrets:

```bash
# JWT secret (64 chars)
openssl rand -hex 32

# HMAC key (32 chars)
openssl rand -hex 16

# AES-128 key for 2FA encryption (32 hex chars = 16 bytes)
openssl rand -hex 16
```

---

## Step 6: Build and Start the Stack

```bash
cd ~/jingdezhen-ceramics-refactor/backend

# Build all images (Go API, worker, frontend SSR)
docker compose -f docker-compose.prod.yml --env-file .env.prod build

# Apply database migrations
# (The API container has the migrations embedded; run via the migrate CLI or
# use the Makefile target if you've installed the migrate CLI on the host)
docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm api /app/server

# Start all services
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d

# Check status
docker compose -f docker-compose.prod.yml ps
```

**The first startup will take a few minutes** — Docker builds the Go binary, the Node SSR bundle, and downloads the PostgreSQL/Redis/Caddy/Prometheus/Grafana images.

### Verify each service:

```bash
# API health
curl http://localhost:1323/
# → {"message":"Welcome to the Jingdezhen Ceramics Platform!"}

# Metrics endpoint
curl http://localhost:1323/metrics
# → # HELP http_requests_total ...

# Public catalog
curl http://localhost:1323/catalog/products?locale=en-US

# Prometheus targets (should show fiber-api as UP)
curl http://localhost:9090/api/v1/targets | python3 -m json.tool | grep health

# Grafana (via SSH tunnel — see Step 8)
```

---

## Step 7: Seed the Database (Optional, for First Deploy)

If you have seed data for dev/demo:

```bash
# Copy seed SQL into the db container and run it
docker compose -f docker-compose.prod.yml --env-file .env.prod exec db psql -U $DB_USER -d $DB_NAME < scripts/seed/00_reset.sql
docker compose -f docker-compose.prod.yml --env-file .env.prod exec db psql -U $DB_USER -d $DB_NAME < scripts/seed/01_users.sql
# ... etc.
```

For production, use real data or migrate from an existing database.

---

## Step 8: Access Grafana and Prometheus

Both Prometheus (port 9090) and Grafana (port 3001) are bound to `127.0.0.1` only — they're not accessible from the public internet. Access them via SSH tunnel:

```bash
# Grafana — open in your browser at http://localhost:3001
ssh -i ~/.ssh/your-key.pem -L 3001:localhost:3001 ubuntu@your-domain.com

# Prometheus — open in your browser at http://localhost:9090
ssh -i ~/.ssh/your-key.pem -L 9090:localhost:9090 ubuntu@your-domain.com

# Or tunnel both at once:
ssh -i ~/.ssh/your-key.pem -L 3001:localhost:3001 -L 9090:localhost:9090 ubuntu@your-domain.com
```

**Grafana login:** The username and password are set by `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD` in `.env.prod`. The "Jingdezhen API Overview" dashboard is auto-provisioned — you'll see it on the dashboards page.

**Prometheus targets:** Navigate to Status → Targets in Prometheus to verify the `fiber-api` job is `UP`.

---

## Step 9: Set Up Nightly Backups

The backup scripts upload `pg_dump` output to Alibaba Cloud OSS. To enable nightly backups on the EC2 instance:

```bash
# Install the AWS CLI (if using S3 instead of OSS)
sudo apt install awscli -y
# Or install ossutil for Alibaba Cloud OSS:
# curl -o /usr/local/bin/ossutil https://gosspublic.alicdn.com/ossutil/1.7.12/ossutil64
# chmod +x /usr/local/bin/ossutil

# Set up the cron job
crontab -e
```

Add this line (runs at 3:00 AM UTC nightly):

```cron
0 3 * * * cd /home/ubuntu/jingdezhen-ceramics-refactor/backend && ./scripts/backup/backup.sh >> /var/log/jdz-backup.log 2>&1
```

Or use the Makefile target (if you've set `OSS_*` env vars):

```bash
0 3 * * * cd /home/ubuntu/jingdezhen-ceramics-refactor/backend && make backup >> /var/log/jdz-backup.log 2>&1
```

---

## Step 10: Configure Caddy TLS

Caddy automatically provisions TLS certificates from Let's Encrypt when the domain resolves to the instance. No manual configuration needed — just ensure:

1. **Port 443 is open** in the security group (done in Step 2.2).
2. **DNS is pointed** at the elastic IP (done in Step 3).
3. **The `SITE_DOMAIN` env var** in `.env.prod` matches your domain.

Caddy will:
- Provision a certificate on first startup (~30 seconds).
- Auto-renew certificates 30 days before expiry.
- Redirect all HTTP (port 80) traffic to HTTPS (port 443).

If TLS provisioning fails, check `docker compose -f docker-compose.prod.yml logs caddy`.

---

## Step 11: Set Up the Migrate CLI (for Schema Changes)

Install the `migrate` CLI on the EC2 instance for future schema migrations:

```bash
# Install golang-migrate
curl -L https://packagecloud.io/golang-migrate/migrate/gpgkey | sudo apt-key add -
echo "deb https://packagecloud.io/golang-migrate/migrate/any/ any main" | sudo tee /etc/apt/sources.list.d/golang-migrate.list
sudo apt update
sudo apt install migrate -y

# Or via Go:
# go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Set `DATABASE_URL` in the environment and run migrations:

```bash
export DATABASE_URL="postgres://jingdezhen:<password>@localhost:5432/jingdezhen_ceramics_db?sslmode=disable"
cd ~/jingdezhen-ceramics-refactor/backend
make migrate-up
```

---

## Operating the Stack

### Common commands

```bash
cd ~/jingdezhen-ceramics-refactor/backend

# View logs (all services)
docker compose -f docker-compose.prod.yml logs -f

# View logs (specific service)
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml logs -f worker
docker compose -f docker-compose.prod.yml logs -f caddy
docker compose -f docker-compose.prod.yml logs -f prometheus

# Restart a service
docker compose -f docker-compose.prod.yml restart api

# Scale the API (if needed — but the MVP is single instance)
# docker compose -f docker-compose.prod.yml up -d --scale api=2

# Stop everything
docker compose -f docker-compose.prod.yml down

# Stop + wipe data (DANGER — deletes PostgreSQL + Redis data)
# docker compose -f docker-compose.prod.yml down -v
```

### Health checks

```bash
# API
curl https://your-domain.com/api/              # via Caddy (public)
curl http://localhost:1323/                   # direct (internal)

# Metrics
curl http://localhost:1323/metrics | head -20

# Prometheus targets
curl http://localhost:9090/api/v1/targets | python3 -m json.tool

# Container status
docker compose -f docker-compose.prod.yml ps
```

### Updating the app

```bash
cd ~/jingdezhen-ceramics-refactor
git pull origin main

cd backend
docker compose -f docker-compose.prod.yml --env-file .env.prod build
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d

# Run any new migrations
make migrate-up
```

---

## Monitoring Dashboard

The auto-provisioned Grafana dashboard ("Jingdezhen API Overview") shows:

| Panel | Metric | What it tells you |
|-------|--------|-------------------|
| Request Rate | `rate(http_requests_total[1m])` | Requests per second |
| p95 Latency | `histogram_quantile(0.95, ...)` | 95th percentile response time |
| Error Rate | `5xx / total` | Server error percentage |
| In-Flight | `http_requests_in_flight` | Concurrent active requests |
| Latency by Route | p95 per route | Which routes are slow |
| Rate by Status | per status code | 2xx/4xx/5xx breakdown |
| Rate by Method | per HTTP method | GET/POST/PUT/DELETE distribution |

**Prometheus scrape interval:** 15 seconds. **Grafana refresh:** 15 seconds.

---

## Security Hardening Checklist

- [ ] Security group: only ports 22 (SSH from your IP), 80, 443 open.
- [ ] `.env.prod` permissions: `chmod 600 .env.prod` (only the ubuntu user can read it).
- [ ] Grafana admin password changed from the default.
- [ ] `JWT_SECRET` is a 64+ character random string.
- [ ] `DB_PASSWORD` is a strong random password (not in the repo).
- [ ] SSH key-based auth only (disable password auth in `/etc/ssh/sshd_config`).
- [ ] `fail2ban` installed for SSH brute-force protection: `sudo apt install fail2ban`.
- [ ] Docker daemon configured to only listen on the Unix socket (not TCP).
- [ ] Prometheus + Grafana bound to `127.0.0.1` (not `0.0.0.0`) — they are in the compose file.
- [ ] Unattended security upgrades: `sudo apt install unattended-upgrades && sudo dpkg-reconfigure -plow unattended-upgrades`.

---

## Troubleshooting

### Caddy TLS fails
```bash
docker compose -f docker-compose.prod.yml logs caddy
```
Check that DNS resolves to the elastic IP (`dig your-domain.com`), port 443 is open in the security group, and `SITE_DOMAIN` in `.env.prod` matches.

### API won't start
```bash
docker compose -f docker-compose.prod.yml logs api
```
Common causes: database connection (check `DATABASE_URL`), Redis connection (check `REDIS_URL`), JWT secret missing.

### Prometheus can't scrape the API
```bash
# Check the target status
curl http://localhost:9090/api/v1/targets | python3 -m json.tool

# Check the /metrics endpoint directly
curl http://localhost:1323/metrics
```
The Prometheus container reaches `api:1323` over the Docker network — if the API container isn't running or isn't on the same network, scraping will fail.

### Grafana shows no data
1. Verify Prometheus is scraping (Step 8).
2. Check the datasource is configured: Settings → Data Sources → Prometheus → Save & Test.
3. Ensure the time range in the top-right is set to "Last 1 hour" (not a future range).

### Database connection errors
```bash
# Check PostgreSQL is healthy
docker compose -f docker-compose.prod.yml exec db pg_isready -U $DB_USER -d $DB_NAME

# Check the DATABASE_URL in the api container
docker compose -f docker-compose.prod.yml exec api env | grep DATABASE_URL
```

---

## Cost Estimate (AWS EC2, us-east-1, on-demand)

| Resource | Cost/mo |
|----------|---------|
| `t3.large` EC2 (2 vCPU, 8 GB) | ~$60 |
| 30 GB gp3 EBS | ~$2.50 |
| Elastic IP (attached) | $0 |
| Data transfer (first 100 GB free) | $0 |
| Route 53 hosted zone (optional) | $0.50 |
| **Total** | **~$63/mo** |

For a cheaper MVP, use `t3.medium` (~$30/mo + $2.50 EBS = ~$33/mo). Use a **Reserved Instance** or **Savings Plan** for 30-60% off if you commit to 1-3 years.
