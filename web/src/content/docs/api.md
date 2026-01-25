# API Reference

The Sennet Control Plane exposes a REST/gRPC API for programmatic access to your data.

## Base URL
`https://api.sennet.dev/v1`

## Authentication
All requests must include the `Authorization` header:
`Authorization: Bearer <YOUR_API_KEY>`

## Endpoints

### Get Agents
List all active agents in your fleet.

`GET /agents`

**Response:**
```json
[
  {
    "id": "agent_123",
    "hostname": "prod-web-01",
    "ip": "10.0.1.5",
    "status": "online",
    "version": "0.1.3"
  }
]
```

### Get Metrics
Query aggregated metrics.

`GET /metrics?from=now-1h&to=now`

**Parameters:**
- `query`: PromQL-compatible query string
- `from`: Start timestamp
- `to`: End timestamp

**Response:**
```json
{
  "data": [
    {
       "timestamp": 167888221,
       "value": 4500
    }
  ]
}
```
