<div align="center">
<br/>
<img src="assets/logo.webp" alt="Hytale Docker Logo" width="200" />

# Docker Hytale Server

**A production-ready Docker image for hosting Hytale dedicated servers.**
</div>

## Quick Start

### Using Docker Compose (Recommended)

```yaml
services:
  hytale:
    image: ghcr.io/kipicenko/docker-hytale-server:latest
    container_name: hytale-server
    restart: unless-stopped
    stdin_open: true
    tty: true
    ports:
      - "5520:5520/udp"
    environment:
      - SERVER_NAME=Hytale Server
      - SERVER_MOTD=Welcome to my Hytale server!
      - MAX_PLAYERS=50
      - MAX_VIEW_RADIUS=32
      # See env variables here - ".env.exmaple"
    volumes:
      - ./hytale-data:/data
```

Start the server:
```bash
docker compose up
```

## First-Time Authentication

On first run, you'll need to complete **two OAuth authorizations** (Hytale requirement):

### Why Two Logins?

Hytale uses separate OAuth clients with different scopes:
- `hytale-downloader` - Downloads game files
- `hytale-server` - Authenticates server for player connections

These cannot be combined (Hytale security restriction). **But both credentials are saved** - all future restarts require **zero logins**.


## Token Passthrough (GSP/Hosting Providers)

Skip the interactive auth flow by passing tokens directly:

| Variable                       | Description              |
|--------------------------------|--------------------------|
| `HYTALE_SERVER_SESSION_TOKEN`  | Session token (JWT)      |
| `HYTALE_SERVER_IDENTITY_TOKEN` | Identity token (JWT)     |
| `HYTALE_OWNER_UUID`            | Profile UUID for session |

```yaml
environment:
  HYTALE_SERVER_SESSION_TOKEN: "eyJhbGciOiJFZERTQSIs..."
  HYTALE_SERVER_IDENTITY_TOKEN: "eyJhbGciOiJFZERTQSIs..."
  HYTALE_OWNER_UUID: "123e4567-e89b-12d3-a456-426614174000"
```

## Links

- [Hytale Official Website](https://hytale.com/)
- [Hytale Server Manual](https://support.hytale.com/hc/en-us/articles/45326769420827-Hytale-Server-Manual)
- [Server Provider Authentication Guide](https://support.hytale.com/hc/en-us/articles/45328341414043-Server-Provider-Authentication-Guide)

## License
Copyright © 2026, [Aleksey Kipichenko](https://github.com/Kipicenko).
Released under the [MIT License](LICENSE).
