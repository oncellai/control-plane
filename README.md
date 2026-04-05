# oncell-control-plane

The Control Plane manages cell lifecycle, scheduling, routing, and metrics. It sits between the API Server and the Data Plane (Host Agents). Never exposed to the public internet.

## Architecture

```
API Server (TypeScript, Fargate)
     │
     │ internal HTTP
     ▼
┌─────────────────────────────────────────────────────┐
│                  Control Plane                       │
│                                                      │
│   Cell Manager     Create, pause, wake, delete cells │
│   Scheduler        Pick host for new cells           │
│   Router           customer_id → host:port mapping   │
│   Idle Checker     Pause cells after timeout         │
│   Health Monitor   Detect dead hosts                 │
│                                                      │
│   Tech:    Go                                        │
│   State:   ElastiCache Redis                         │
│   Comms:   gRPC to Host Agents                       │
│   Deploy:  ECS Fargate (private subnet)              │
└──────────────────────┬──────────────────────────────┘
                       │ gRPC
                       ▼
               Host Agents (Rust, EC2)
```

## Internal API (called by API Server only)

```
POST   /cells/create     Create a cell on a host
POST   /cells/pause      Pause a cell (free CPU/RAM, keep NVMe)
POST   /cells/resume     Resume a paused cell
POST   /cells/delete     Delete a cell and all data
GET    /cells/:id/status Cell status + host info
GET    /cells/route/:customer_id  Get routing info (host:port)
GET    /hosts             List hosts with metrics
GET    /health            Health check
```

## Core Modules

| Module | What It Does |
|--------|-------------|
| `cellmanager` | Cell lifecycle state machine — create, pause, resume, delete |
| `scheduler` | Host selection — filter by capacity, prefer NVMe cache |
| `router` | Redis routing table — customer_id → host:port |
| `idlechecker` | Background goroutine — pause cells idle > timeout |
| `healthmonitor` | Detect dead hosts — failover cells to healthy hosts |
| `hostclient` | gRPC client to Host Agents |

## Idle Cell Reclamation

```
Three mechanisms:

1. Idle Checker (every 30 seconds):
   Scan Redis for active cells where now - last_active_at > idle_timeout
   → Tell Host Agent to pause
   → Update routing table

2. Heartbeat:
   API Server updates last_active_at on every request
   SDK sends heartbeat during long-running operations
   Keeps cell alive during 45-min test suites

3. Force Reclaim (host is full):
   New cell needs capacity but host is full
   → Find least recently active cell on that host
   → Pause it (5-min grace period, not 30-min)
   → Create new cell in freed capacity
```

## Scheduler Logic

```
Input: "I need a cell with 4cpu, 8gb, 50gb NVMe"

Step 1 — Filter: which hosts have enough free resources?
  Host 1: 12/16 cpu used → 4 free ✓
  Host 2: 15/16 cpu used → 1 free ✗

Step 2 — Prefer: any host already has this customer's NVMe data?
  Host 1: has /cells/acme-corp/ cached → CACHE HIT ✓

Step 3 — Tiebreak: least loaded host
  If multiple cache hits or no cache: pick lowest cpu_used

Result: Host 1 (cache hit + capacity)
```

## Data Safety

```
Cell data lives on NVMe (Host Agent manages it).
Control Plane ensures safety through:

1. Snapshot on pause:
   Before marking a cell as paused, tell Host Agent to snapshot to S3.
   Blocking — cell not marked paused until snapshot completes.

2. Periodic snapshots:
   Every hour, tell Host Agent to snapshot active cells.
   Background — doesn't block cell operations.

3. Host failure recovery:
   Health Monitor detects dead host (no heartbeat for 30s).
   For each cell on that host:
   → Download latest S3 snapshot to a new host
   → Resume cell, replay journal
   → Update routing table

4. Snapshot retention:
   Keep last 3 snapshots per cell in S3.
   Delete older ones to save storage cost.
```

## Redis Schema

```
# Routing table (used by API Server via Control Plane)
cell:{cell_id}:route → {"host": "10.0.20.5", "port": 8401, "status": "active"}

# Host metrics (pushed by Host Agents every 5 seconds)
host:{host_id}:metrics → {
  "cpu_total": 16000,
  "cpu_used": 12000,
  "ram_total": 68719476736,
  "ram_used": 50000000000,
  "active_cells": 8,
  "paused_cells": 42,
  "cached_customers": ["acme-corp", "initech", ...]
}
host:{host_id}:heartbeat → timestamp (TTL: 30s)

# Cell activity tracking
cell:{cell_id}:last_active → timestamp
```

## Prerequisites

- Go 1.22+
- Redis (local for dev, ElastiCache for prod)
- protoc + protoc-gen-go + protoc-gen-go-grpc (for Host Agent gRPC client)

## Setup

```bash
go mod tidy
cp .env.example .env

# Development
go run ./cmd/control-plane

# Production
go build -o control-plane ./cmd/control-plane
./control-plane
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `PORT` | `4000` | HTTP server port |
| `REDIS_URL` | `localhost:6379` | Redis connection |
| `IDLE_TIMEOUT_SECS` | `1800` | Seconds before idle cell is paused |
| `HEALTH_CHECK_INTERVAL` | `10` | Seconds between host health checks |
| `SNAPSHOT_INTERVAL_SECS` | `3600` | Seconds between periodic snapshots |

## Deploy

```bash
docker build -t oncell-control-plane .
docker tag oncell-control-plane:latest $ECR_REPO/oncell-control-plane:latest
docker push $ECR_REPO/oncell-control-plane:latest
aws ecs update-service --cluster oncell --service control-plane --force-new-deployment
```
