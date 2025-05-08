# Mattermost Release Notes Extractor

This tool helps extract release notes from GitHub pull requests in Mattermost repositories. It retrieves PRs with the "release-note" label from selected milestones and displays their release notes.

## Prerequisites

### GitHub API Token

You need a GitHub API token with appropriate permissions to access the repositories:

1. Go to your GitHub account settings
2. Select "Developer settings" from the left sidebar
3. Click on "Personal access tokens" > "Tokens (classic)"
4. Click "Generate new token" > "Generate new token (classic)"
5. Give your token a name and select the following scopes:
   - `repo` (Full control of private repositories)
6. Click "Generate token"
7. Copy the token (you won't be able to see it again!)

#### Using Token with SAML Authentication

If your GitHub organization uses SAML SSO (Single Sign-On):

1. After creating your token, go to the token's page
2. Under "Organization access", find your organization
3. Click "Configure SSO"
4. Click "Authorize" for your organization
5. Complete the SAML authentication process if prompted

## Installation and Usage

1. Install the tool using Go:
   ```
   go install github.com/jespino/github-mm-release-notes@latest
   ```

2. Run the tool with your GitHub token in one of these ways:

   **Command line flag (preferred):**
   ```
   github-mm-release-notes --token=YOUR_TOKEN_HERE
   ```

   **Environment variable:**
   ```
   export GITHUB_TOKEN=YOUR_TOKEN_HERE
   github-mm-release-notes
   ```

   **Note:** The tool will fall back to using any token stored in the code (if any), but this is not recommended.
   
   **Important:** If you don't provide a GitHub token, the tool will run as an unauthenticated user. This will work for the public Mattermost repository, but you **won't be able to access** the private Enterprise repository.

3. Follow the interactive prompts:
   - Select a repository (mattermost/mattermost, mattermost/enterprise, mattermost/mattermost-mobile, mattermost/mattermost-desktop, mattermost/mattermost + mattermost/enterprise, or all)
   - Select a milestone from the displayed list
   - The tool will display all PRs with the "release-note" label in that milestone

## Supported Release Note Formats

The tool attempts to extract release notes from PR descriptions in several formats:

- Code blocks with release-note tag:
  ```release-note
  Your release note here
  ```

- Markdown section titled "Release Note"
- Simple "release-note:" prefix
- Any paragraph mentioning "release note"
