# Campus Monitor API

Backend for the Campus Network Monitoring System – collects telemetry from IoT probes via MQTT, stores data in TimescaleDB, and provides a REST API for the web dashboard.

## Features

- MQTT telemetry ingestion (light and enhanced)
- Real‑time alert evaluation (RSSI, latency, packet loss)
- Fleet management (enroll probes, groups, config templates)
- Command dispatch (deep scan, OTA, reboot) to individual probes or fleet
- REST API with JWT authentication and optional LDAP/OAuth
- WebSocket for live alerts
- Report generation (PDF/JSON)

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+ with TimescaleDB extension
- Mosquitto MQTT broker

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourorg/campus-monitor-api.git
   cd campus-monitor-api
   ```
2. Copy .env.example to .env and edit the values:
```bash

cp .env.example .env
nano .env
```
3. Build and run:
```bash

go build -o campus-monitor-api ./cmd/api
./campus-monitor-api
```
Or use Docker:
```bash

docker build -t campus-monitor-api .
docker run -p 8080:8080 --env-file .env campus-monitor-api
```

### Configuration
All configuration is done via environment variables (see .env.example). Required variables are marked.