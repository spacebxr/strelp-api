# Testing the GitHub Poller

This guide explains how to get a GitHub Personal Access Token and verify that the GitHub polling system is detecting your commits correctly. It is written for contributors mapping out their development environments for the first time.
or if you wanna try it out, thats fine.

---

## Step 1: Get a GitHub Personal Access Token

The poller needs a token to read your GitHub activity on your behalf.

### Classic PAT (Recommended for simplicity)

1. Go to github.com/settings/tokens and generate a new classic token.
2. Give it an easily identifiable name.
3. Set an expiration that suits you for your testing period.
4. Under the Scopes section, tick `repo` (if you want to track private repositories) and `read:user` (to verify your username). If you only want to track public repos, `public_repo` is enough.
5. Generate the token and copy it immediately, as GitHub will not show it again.

### Fine-Grained PAT

1. Go to your GitHub developer settings and generate a new fine-grained token.
2. Under Repository access, choose either "All repositories" or select specific ones. You must do this first, or the necessary permission options will not be visible.
3. Once repository access is set, scroll down and set `Contents` and `Metadata` permissions to Read-only.
4. Generate and copy the token.

---

## Step 2: Link Your Token in Discord

Once you have your token, head to the Discord server where the bot is running and use the `/git` command.

You will provide your token and specify your visibility preference, determining whether you want the bot to track public pushes, private pushes, or both. If the token is valid, the bot will confirm it has been successfully encrypted and saved alongside your username.

---

## Step 3: Trigger the Poller

The poller ordinarily runs completely automatically in the background every five minutes. If you want to see it work immediately without waiting:

1. Make a real commit to one of your tracked repositories and push it to GitHub.
2. Restart the API server locally. When the API service boots up, the poller executes an initial check right away before entering its five-minute loop.

You will see a log line in your terminal indicating how many users are being polled.

Because GitHub removed commit messages natively from their public Events API payloads, the poller now parses the push event for the latest commit SHA and then makes a secondary, authenticated REST API call to directly pull your commit message.

---

## Step 4: Verify the Result

Once the poller has finished running, hit your local presence endpoint in your browser or a tool like cURL:

`GET http://localhost:<PORT>/v1/presence/<YOUR_DISCORD_USER_ID>`

In the JSON response, you should see a `github` object populated with your repository name, the specific commit message that was fetched, whether it was a private repository, and a timestamp. 

---

## Troubleshooting

- If you see "Failed to decrypt token" in your logs, the `ENCRYPTION_KEY` environment variable has likely changed since the token was first stored. Run the stop command in Discord and link your account again.
- A 401 error means your token has expired or been revoked by GitHub.
- A 403 error means your token does not have the necessary scopes. This is common if you are using a fine-grained token but failed to give it Contents and Metadata permissions.
- If the github field is entirely absent after the poller runs, ensure you actually pushed a commit after linking your account, and that your visibility settings match the repository type you pushed to.
