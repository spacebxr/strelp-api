# Privacy Policy

Last Updated: April 17, 2026

At Strelp, we believe in being completely transparent about what data we handle and exactly how we keep it secure.

## Data We Collect

We only collect and process data that is strictly necessary for our presence service to function:
- Discord ID: We need this to uniquely identify your presence profile and settings.
- Presence Data: We track your online status, current games, your active devices, and activity metadata.
- Public Visual Profile: Through our DSTN api integration, we process your public Discord badges, active clan tags, and nameplate decorations.
- Spotify Data: We track the current track, artist, and album information if you are listening to Spotify via your connected account.
- GitHub Data: If you explicitly link your account using the git command, we store your GitHub username and continuously fetch your latest repository commit messages to display.
- GitHub Tokens: We store your Personal Access Token only if you actively provide it.

## How We Store Your Data

All presence data is stored in our database and updated continuously in real-time.

Your GitHub Personal Access Tokens are heavily encrypted using the AES-256-GCM cipher before they are ever saved. We never store them anywhere in plain text. We do not log your GitHub tokens in the application console, and we never include them in any public API responses.

## Data Sharing

Your presence data, including your status, activities, Spotify data, public visual profile, and public GitHub commits, is actively shared through our public API endpoint and WebSockets. This is the core operating purpose of Strelp. We absolutely do not sell your data to third parties or use it for advertising purposes.

## Data Deletion and Control

You have full control over your data at any time:
- Stop Tracking: Run the base stop command to immediately and permanently delete your presence data from our database.
- Disconnect GitHub: Run the github stop command to permanently delete your GitHub settings and physically erase the encrypted token.
- Automatic Deletion: If you leave our Discord server, our system will recognize the exit event and automatically purge your data.

## Security

We take security seriously and utilize industry-standard encryption for sensitive data like tokens. However, no internet service is entirely foolproof, and we strongly encourage you to prioritize fine-grained GitHub tokens restricted to the absolute bare minimum scopes necessary for the bot to run.

## Official Service Only

This policy applies exclusively to the official public Strelp bot and API. We have no control over, and assume no responsibility for, the privacy or security of any data you provide to self-hosted versions of Strelp or third-party bots using our open-source codebase.

## Contact

If you have questions about your data or our privacy practices, please contact our developers via the main server.
