# Privacy Policy

Last Updated: April 15, 2026

At Strelp, we believe in being transparent about what data we handle and how we keep it secure.

## Data We Collect
We only collect data that is necessary for the service to function:
- Discord ID: To identify your presence and settings.
- Presence Data: Your online status, current games, and activity metadata.
- Spotify Data: Current track, artist, and album information if you are listening to Spotify.
- GitHub Data: If you link your account, we store your GitHub username and your latest commit information.
- GitHub Tokens: We store your Personal Access Token only if you provide it for the /git command.

## How We Store Your Data
- Presence data is stored in our database and updated in real-time.
- GitHub Personal Access Tokens are encrypted using AES-256-GCM before being saved. We never store them in plain text.
- We do not log your GitHub tokens or include them in any API responses.

## Data Sharing
Your presence data (status, activities, Spotify, and public GitHub commits) is shared through our public API. This is the core purpose of Strelp. We do not sell your data to third parties or use it for advertising.

## Data Deletion and Control
You have full control over your data:
- Stop Tracking: Run the /stop command to delete your presence data from our database.
- Disconnect GitHub: Run the /gitstop command to permanently delete your GitHub settings and encrypted token.
- Automatic Deletion: If you leave our Discord server, our system will automatically purge your data within a short period.

## Security
We take security seriously and use industry-standard encryption for sensitive data like tokens. However, no service is 100% secure, and we encourage you to use fine-grained GitHub tokens with limited scopes.

## Official Service Only
This policy applies exclusively to the official Strelp bot and API. We have no control over, and assume no responsibility for, the privacy or security of any data you provide to self-hosted versions of Strelp or third-party bots using our code or API.

## Contact
If you have questions about your data or our privacy practices, please contact us through our Discord server.
