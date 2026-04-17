# Strelp Presence API

Strelp is a high-performance, real-time presence API that tracks user activity across Discord, GitHub, and Spotify. It is powered by Go and PostgreSQL, ensuring minimal latency and extremely reliable data persistence using JSONB and Listen/Notify patterns.

## Features

The API aims to consolidate your digital presence into one clean endpoint.

It offers real-time streaming capability via WebSockets, so any applications or websites you build can listen to your presence live without needing to constantly poll the server.

The core Discord integration securely connects your profile to pull down your current status, active devices (desktop, web, or mobile), rich presence activities, and live Spotify playback details. Additionally, we use the DSTN integration to display aesthetic profile details like your Discord badges, clan tags, and custom nameplates.

Importantly, tracking is heavily opt-in. The bot will only track and expose data for users who explicitly initialize tracking by running the start command.

## Deployment on Railway

This project is built from the ground up to be deployed on Railway.

To get started, link your GitHub repository to a new Railway project and deploy two separate services from this codebase: one for the bot and one for the API server. You must set the start command for the bot service to `./bot` and the start command for the API service to `./api`.

### Environment Variables

You must add the following variables in your Railway deployment for both services to run correctly:

- `DISCORD_TOKEN`: Your standard Discord Bot Token.
- `DATABASE_URL`: Your PostgreSQL connection string. 
- `ENCRYPTION_KEY`: A secret string (minimum 16 characters) used to securely encrypt your users' GitHub Personal Access Tokens in the database.
- `GUILD_ID`: The Discord Server ID where the bot will operate. This is strictly required to lock the bot's functionality and tracking to a single specific server.
- `SYNC_ROLES`: A comma-separated list of Discord Role IDs. Server staff with these roles can use administrative synchronization commands.
- `RAILWAY_PUBLIC_DOMAIN`: Set this in your API service to let the bot construct correct endpoint URLs for your users.

## Administrative Sync

Whenever structural updates or API migrations are deployed, users normally need to re-run the start command to refresh their profiles. Staff members holding any role defined in the `SYNC_ROLES` variable can instead run the `/sync` command to forcefully update all currently tracked users to the latest schema version cleanly and concurrently.

## Local Testing

If you want to run Strelp locally for development:
1. Copy the `.env.example` file to `.env` and fill in your variables.
2. Build the binaries using `go build -o bot ./cmd/bot` and `go build -o api ./cmd/api`.
3. Run `./bot` and `./api` in separate terminal windows.

## API Usage

The main HTTP endpoint to retrieve a user's populated data is:
`GET /v1/presence/{discordUserID}`

If you want instant WebSocket updates pushed to your client side, connect to:
`WSS /v1/presence/{discordUserID}/ws`

Take a look at the `/docs` directory in this repository to read about our privacy policies, terms of service, and testing the GitHub Activity poller.

---

Built by spacebxr, sachfax, and other contributors. 
If this project helped you, starring it on GitHub is greatly appreciated.
