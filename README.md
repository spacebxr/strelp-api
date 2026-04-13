# Strelp Presence API

High-performance, real-time presence API tracking user activity across Discord, GitHub, and Spotify. Powered by Go and PostgreSQL.

## Features
- **Real-time Streaming**: WebSocket updates for live presence data.
- **Discord Integration**: Tracks status, activities, and Spotify.
- **PostgreSQL Engine**: Reliable persistence with JSONB and LISTEN/NOTIFY.
- **Opt-in Only**: Only tracks users who explicitly run `/start`.

## Deployment (Railway)

This project is optimized for deployment on [Railway](https://railway.app).

### 1. Create a Railway Project
Link your GitHub repository to a new Railway project.

### 2. Configure Services
You can deploy two services from this codebase:
- **Bot Service**:
  - Start Command: `./bot`
- **API Service**:
  - Start Command: `./api`

### 3. Environment Variables
Add the following variables in the Railway Dashboard:
- `DISCORD_TOKEN`: Your Discord Bot Token.
- `DATABASE_URL`: Your PostgreSQL connection string (e.g., from Supabase).
- `GUILD_ID`: (Optional) For instant command registration.

## Local Testing

1. Copy `.env.example` to `.env` and fill in the values.
2. Build the binaries:
   ```bash
   go build -o bot ./cmd/bot
   go build -o api ./cmd/api
   ```
3. Run the services:
   ```bash
   ./bot
   ./api
   ```
built with :heart: by [spacebxr](https://spacebxr.pages.dev) and other contributors.
A github star would be much appreciated.
