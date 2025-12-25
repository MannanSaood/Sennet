# Sennet Backend Deployment Guide

## Koyeb Deployment (Recommended - Free Tier)

### Prerequisites
1. [Koyeb account](https://app.koyeb.com/) (free tier available)
2. GitHub repository connected

### Option A: Deploy via Web UI (Easiest)

1. Go to [Koyeb Dashboard](https://app.koyeb.com/)
2. Click **Create App** → **GitHub**
3. Connect your GitHub repo
4. Configure:
   - **Dockerfile path**: `backend/Dockerfile`
   - **Port**: `8080`
   - **Region**: Frankfurt (fra) for free tier
5. Add environment variables:
   - `LATEST_VERSION` = `1.0.0`
6. Click **Deploy**

### Option B: Deploy via CLI

```bash
# Install Koyeb CLI
# Windows (via Scoop):
scoop install koyeb

# Or download from: https://github.com/koyeb/koyeb-cli/releases

# Login
koyeb login

# Deploy (from project root)
koyeb app create sennet-backend \
  --docker backend/Dockerfile \
  --ports 8080:http \
  --routes /:8080 \
  --regions fra \
  --instance-type free \
  --env PORT=8080 \
  --env LATEST_VERSION=1.0.0
```

### Verify Deployment

```bash
# Your URL will be: https://sennet-backend-<your-org>.koyeb.app

# Test health
curl https://sennet-backend-<your-org>.koyeb.app/health

# Test auth (should return 401)
curl -X POST https://sennet-backend-<your-org>.koyeb.app/sentinel.v1.SentinelService/Heartbeat
```

### Generate API Key on Koyeb

Koyeb doesn't have persistent storage on free tier, so you'll need to:

1. Use environment variable for a pre-generated key, OR
2. Upgrade to add persistent volume

For now, let's use an in-memory initial key. Add this environment variable:
- `INIT_API_KEY` = (generate one locally first with `./sennet-server keygen`)

---

## Railway Deployment (Requires Paid Plan)

```bash
# Set via CLI
railway variables set LATEST_VERSION=1.0.0
```

### Persistent Storage

Railway provides ephemeral storage by default. For persistent SQLite:

1. Go to Railway Dashboard → Your Project → Settings
2. Add a Volume mounted at `/data`
3. The database will persist at `/data/sennet.db`

### Generate API Key (After Deploy)

```bash
# SSH into the container
railway run ./sennet-server keygen --name "Production"
```

Or use the Railway shell:
```bash
railway shell
./sennet-server keygen --name "Production"
```

### Verify Deployment

```bash
# Get your deployment URL
railway open

# Test health endpoint
curl https://your-app.railway.app/health

# Test heartbeat (should return 401 without auth)
curl -X POST https://your-app.railway.app/sentinel.v1.SentinelService/Heartbeat
```

---

## Fly.io Deployment (Alternative)

### Prerequisites
1. [Fly CLI](https://fly.io/docs/hands-on/install-flyctl/) installed
2. Fly.io account

### Quick Deploy

```bash
# Login
fly auth login

# Launch (first time)
cd backend
fly launch --dockerfile Dockerfile

# Deploy updates
fly deploy
```

### fly.toml

Create `backend/fly.toml`:

```toml
app = "sennet-backend"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  LATEST_VERSION = "1.0.0"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0

[[http_service.checks]]
  path = "/health"
  interval = "30s"
  timeout = "5s"

[mounts]
  source = "sennet_data"
  destination = "/data"
```

### Create Volume

```bash
fly volumes create sennet_data --size 1 --region sjc
```

### Deploy

```bash
fly deploy
```

---

## Docker Local Testing

```bash
# Build
docker build -t sennet-backend -f backend/Dockerfile .

# Run
docker run -p 8080:8080 -v sennet-data:/data sennet-backend

# Test
curl http://localhost:8080/health
```
