# API Documentation

Base URL: `http://localhost:8080/api/v1` (configurable via `SERVER_PORT`)

All endpoints except `/auth/login`, `/auth/register`, `/auth/refresh`, `/auth/config`, and OAuth callbacks require a Bearer token in the `Authorization` header.

## Authentication

### POST `/auth/register`
Register a new user.

**Request body:**
```json
{
  "username": "string",
  "email": "string",
  "password": "string",
  "role": "admin" (optional)
}
```
### POST /auth/login

Login with username/password.

Request body:
```json

{
"username": "string",
"password": "string"
}
```

Response:
- If 2FA not required:`{"access_token": "...", "refresh_token": "..."}`
- If 2FA required: `{"2fa_required": true, "temp_token": "..."}`

### GET /auth/config

Returns dynamic authentication configuration (local login enabled, registration, OIDC providers, admin registration).
### POST /auth/refresh

Refresh access token.

Request body: `{"refresh_token": "..."}`
### POST /auth/logout

Revoke refresh token.


## Probes

### GET /probes

List all probes.
### GET /probes/{id}

Get a specific probe.
### POST /probes

Create a new probe.

Request body: `{"probe_id": "...", "location": "...", "building": "...", "floor": "...", "department": "..."}`
### PUT /probes/{id}

Update probe.
### DELETE /probes/{id}

Delete probe.
### POST /probes/{id}/command

Send a command to a probe.

Request body: `{"command_type": "deep_scan", "payload": {"duration": 5}}`
### GET /probes/{id}/status

Get live status from the probe (cached).
### GET /probes/{id}/config

Get probe configuration (cached).


## Telemetry
### GET /telemetry

Query telemetry with filters.

Query parameters:

    probe_id (repeatable)

    type (light/enhanced)

    start_time (RFC3339)

    end_time (RFC3339)

    limit, offset

### GET /telemetry/{probe_id}/latest?limit=10

Get latest telemetry for a probe.
### GET /telemetry/{probe_id}/stats?hours=24

Get hourly aggregated stats.


## Analytics
### GET /analytics/timeseries/rssi

RSSI time series. Parameters: probe_id, start_time, end_time, interval (e.g., "1 hour").
### GET /analytics/timeseries/latency

Same as above for latency.
### GET /analytics/heatmap

Signal strength heatmap by building/floor.
### GET /analytics/channels

Channel distribution.
### GET /analytics/aps

Access point analysis (top APs).
### GET /analytics/congestion

Congestion analysis over time.
### GET /analytics/performance/{probe_id}

Performance metrics (average RSSI, latency, packet loss, percentiles).
### GET /analytics/comparison?probe_ids=id1&probe_ids=id2&hours=24

Compare multiple probes.
### GET /analytics/health

Network health overview.
### GET /analytics/anomalies/{probe_id}?hours=24

Detect anomalies using standard deviation.
### GET /analytics/roaming/{probe_id}?start_time=...&end_time=...

AP transition history for a probe.
### GET /analytics/coverage?probe_id=...&start_time=...&end_time=...

Daily data coverage (true/false per day).


## Alerts
### GET /alerts/active

All active alerts.
### GET /alerts/history?limit=50&offset=0

Alert history (active and resolved).
### GET /alerts/probe/{probe_id}

Alerts for a specific probe.
### PUT /alerts/acknowledge/{id}

Acknowledge an alert.
### PUT /alerts/resolve/{id}

Resolve an alert.
### DELETE /alerts/{id}

Delete an alert.
### POST /alerts/test

Send a test alert (admin only).


## Commands
### POST /commands

Issue a command to a single probe.

Request body: `{"probe_id": "...", "command_type": "...", "payload": {...}}`
### GET /commands/probe/{probe_id}?limit=20

Command history for a probe.
### GET /commands/pending

List pending commands.
### POST /commands/broadcast

Broadcast a command to all probes (admin only).

Request body: `{"command_type": "...", "params": {...}}`
### GET /commands/statistics

Command success/failure statistics.
### DELETE /commands/{id}

Delete a command record.


## Fleet Management
### GET /fleet/status

Fleet overview (managed probes, groups, templates, active rollouts).
### GET /fleet/probes?group=...

List fleet-managed probes (optionally filtered by group).
### GET /fleet/unenrolled-probes

Probes not yet enrolled in fleet management.
### POST /fleet/probes/{id}/enroll

Enroll a probe into fleet management.

Request body: `{"groups": [...], "location": "...", "tags": {...}, "config_template_id": 1}`
### POST /fleet/probes/{id}/unenroll

Remove probe from fleet management.
### GET /fleet/probes/{id}

Get fleet probe details.
### PUT /fleet/probes/{id}

Update fleet probe metadata.
### GET /fleet/groups

List fleet groups.
### POST /fleet/groups

Create a group.
### DELETE /fleet/groups/{id}

Delete a group (only if no probes assigned).
### GET /fleet/templates

List config templates.
### GET /fleet/templates/{id}

Get a template.
### POST /fleet/templates

Create a template.
### POST /fleet/templates/{id}/apply

Apply template to probes.

Request body: `{"probe_ids": [...]}`
### DELETE /fleet/templates/{id}

Delete a template.
### POST /fleet/commands

Send a fleet command (target groups, probes, all).

Request body: `{"command_type": "...", "target_all": true, "groups": [...], "probe_ids": [...], "strategy": "immediate", ...}`
### GET /fleet/commands?status=...&limit=50

List fleet commands.
### GET /fleet/commands/{id}

Get rollout status.
### POST /fleet/commands/{id}/cancel

Cancel a fleet command.


## Reports
### GET /reports/generate

Generate a report (PDF or JSON). Parameters: type, format, from, to, probe_ids, building, floor.

Supported types: alerts, analytics, fleet, probes, compliance, firmware_version, outage, command_success, network_baseline, site_survey.
Topology
### GET /topology/layout

Get building/floor/probe tree with coordinates.
### GET /topology/heatmap?metric=rssi

Heatmap data for visualisation.
### GET /topology/building/{building}/floor/{floor}

Detailed probe list for a floor.


## Scheduled Tasks
### GET /probes/{probe_id}/tasks

List scheduled tasks for a probe.
### POST /probes/{probe_id}/tasks

Create a scheduled task.

Request body: `{"command_type": "...", "payload": {...}, "schedule": {"type": "recurring", "cron": "@daily"}}`
### GET /probes/{probe_id}/tasks/{task_id}

Get a task.
### PUT /probes/{probe_id}/tasks/{task_id}

Update a task.
### DELETE /probes/{probe_id}/tasks/{task_id}

Delete a task.


## WebSocket

Connect to `ws://localhost:8080/api/v1/ws` (or wss) with a valid token to receive real‑time alerts. The server sends JSON messages of type Alert.
Error Responses

All errors follow this format:
```json
{
"error": "description"
}
```
HTTP status codes: 400 (bad request), 401 (unauthorized), 403 (forbidden), 404 (not found), 409 (conflict), 500 (internal error).