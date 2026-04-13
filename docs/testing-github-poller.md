# Testing the GitHub Poller

This guide explains how to get a GitHub Personal Access Token and verify that the GitHub poller is working correctly. It is written for contributors setting things up for the first time.

---

## Step 1: Get a GitHub Personal Access Token

The poller needs a token to read your GitHub activity on your behalf. Here is how to get one.

### Classic PAT (Recommended for simplicity)

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens) and click **Generate new token (classic)**.
2. Give it a name like `strelp-dev`.
3. Set an expiration that suits you (30 days is fine for testing).
4. Under **Scopes**, tick the following:
   - `repo` — needed if you want to track private repository commits.
   - `read:user` — needed to verify your username when linking the token.
   - If you only care about public repos, `public_repo` is enough instead of `repo`.
5. Scroll to the bottom and click **Generate token**.
6. Copy the token immediately. GitHub will not show it again.

### Fine-Grained PAT (More secure, slightly more setup)

1. Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and click **Generate new token**.
2. Under **Repository access**, select the specific repositories you want tracked, or choose **All repositories**.
3. Under **Permissions**, set the following to **Read-only**:
   - `Contents`
   - `Metadata`
4. Generate and copy the token.

---

## Step 2: Link Your Token in Discord

Once you have your token, head to the Discord server where the bot is running and use the `/git` command.

- **token**: paste the token you just generated.
- **visibility**: choose what kind of repos you want to appear in your presence.
  - `public` — only public repository pushes.
  - `private` — only private repository pushes.
  - `both` — everything.

If the token is valid, the bot will confirm with a success message and your GitHub username.

---

## Step 3: Trigger the Poller

The poller runs automatically every 5 minutes. To see it work without waiting:

1. Make a commit to one of your repositories and push it to GitHub.
2. Restart the bot locally. On startup, the poller runs an immediate check before entering its 5-minute loop.

```bash
go build -o bot ./cmd/bot && ./bot
```

You should see a log line like this in your terminal shortly after boot:

```
[GitHub] Polling 1 user(s)
```

---

## Step 4: Verify the Result

Once the poller has run, check your presence endpoint to confirm the GitHub data was saved:

```
GET http://localhost:<PORT>/v1/presence/<YOUR_DISCORD_USER_ID>
```

In the JSON response, you should see a `github` field populated with your repo name, the last commit message, and a timestamp. Example:

```json
"github": {
  "username": "your-github-username",
  "repo": "your-org/your-repo",
  "url": "https://github.com/your-org/your-repo",
  "last_commit": "fix: resolve null pointer in presence handler",
  "private": false,
  "updated_at": 1713024000
}
```

If the field is `null` or missing, check the terminal logs for errors. Common issues are covered below.

---

## Troubleshooting

**`[GitHub] Failed to decrypt token for <userID>`**
The `ENCRYPTION_KEY` environment variable might have changed since the token was stored. Run `/gitstop` and link your account again with `/git`.

**`[GitHub] Non-200 response for <username>: 401`**
The token has expired or been revoked. Generate a new one and re-run `/git`.

**`[GitHub] Non-200 response for <username>: 403`**
The token does not have the required scopes. Check Step 1 and make sure the correct permissions are selected.

**`github` field is null after the poller ran**
The poller only picks up `PushEvent` entries. Make sure you pushed an actual commit after linking your account. Also verify your visibility setting matches the repo type (public vs private).
