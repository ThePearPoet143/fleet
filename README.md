# fleet

Sets up and checks the machines AI agents run on, from one Go program.

## The doctor

A read-only check of this machine. It detects the system (`runtime.GOOS`): a Mac is supported, other systems say "not
supported yet". It reads your settings from `fleet/settings.toml` in your system's config folder
(`os.UserConfigDir()`: `~/Library/Application Support` on a Mac), then checks Homebrew, git, gh, Go, Claude Code, T3
Code, that Tailscale is up, that T3 Code's service is current, open to the network and answering, that T3 starts
agents as the bot with a fresh token, that the Mac never sleeps, that repos live in `~/dev`, and that agents have ci's
rules. Each line is a tick, or a cross with the fix; it exits 0 when
all pass.

```
go run ./cmd/fleet doctor
```

Each check is a function tested with sample output (`go test ./...`), on Linux in CI.

## Setup

`fleet setup` asks what this machine is for, whether it's personal or shared, your GitHub username, your agents'
GitHub App (`<username>-agents` unless you say otherwise), the AI tools on it and how it's reached. It detects the tools
and Tailscale or T3 Code, and on a rerun shows each current answer, which Enter keeps. A shared machine stores no way to
reach it. Questions are plain prompts (huh's accessible mode), and each has a flag, so agents can run it unprompted with `--yes`. Without `--yes` it reads answers from standard input, one per
line, so they can be piped in; a piped answer it can't use stops setup, which keeps the answers before it:

```
go run ./cmd/fleet setup --github-user <name> --app-id <id> --tools claude,codex --yes
```

It checks each answer as you give it: the username on GitHub (public `GET /users/{name}`), at once. The app stays
private, and GitHub hides a private app from a public lookup, so setup opens the app's settings page and asks for the
App ID shown there, once (`--app-id`). If an answer fails, setup still saves the ones that passed, and a rerun offers
them again. Each machine has its own app key, made only on the app's page (Credentials, Key pairs, New key). With no key yet,
setup says where to click, waits up to five minutes for the new `.pem` in Downloads, and moves it next to settings,
readable only by you. With a key (taken, or given as `--key`), it signs in as the app (`GET /app` with a JWT) and
stops if the app's name or owner isn't what you answered. Then it finds the app's installation on your account
(`GET /users/{name}/installation`, signed as the app) and keeps its ID; if the app isn't installed yet, it says
where to click Install App and waits. It saves `fleet/settings.toml` in your config folder, readable only by you, then runs the doctor. Settings hold the
app key's path (`--key`), never the key.

## The agents' token

`fleet token` signs in as the app with this machine's key and asks GitHub for an installation token
(`POST /app/installations/{id}/access_tokens`), which lasts an hour. It writes gh's login for the bot,
`fleet/gh/hosts.yml` next to settings, in one step and readable only by you; agents use that folder as
`GH_CONFIG_DIR`. It prints when it refreshed, never the token, and on a failure keeps the old login.

```
go run ./cmd/fleet token
```

Setup ends by running it once, then installs a login job, `dev.fleet.agent-token`, that runs it every 30 minutes
and at login (`StartInterval` 1800, `RunAtLoad`). The job runs a fixed copy of fleet, `fleet/bin/fleet` next to
settings, not a build agents work on, and logs to `fleet/token.log`. Login jobs start only at the Mac, not over SSH
(`launchctl bootstrap gui/<uid>`). The doctor checks that the job runs `fleet token`. Undo:
`launchctl bootout gui/$(id -u)/dev.fleet.agent-token`, then remove `~/Library/LaunchAgents/dev.fleet.agent-token.plist`.

## Agents act as the bot

Setup then makes every agent T3 Code starts act as the app's bot, never as you: it sets five lines in
`~/.t3/service.env`, in place of old values and keeping your other lines, readable only by you. The login folder's
full path is quoted, since T3 doesn't expand `~` and the path has a space; the bot's ID comes from
`GET /users/<app>[bot]`.

```dotenv
GH_CONFIG_DIR="/Users/<you>/Library/Application Support/fleet/gh"
GIT_AUTHOR_NAME=<app>[bot]
GIT_AUTHOR_EMAIL=<bot ID>+<app>[bot]@users.noreply.github.com
GIT_COMMITTER_NAME=<app>[bot]
GIT_COMMITTER_EMAIL=<bot ID>+<app>[bot]@users.noreply.github.com
```

T3 reads the file when it starts, and a restart stops running agents, so setup leaves `t3 service restart` to you, at
the Mac. The doctor checks the lines name fleet's login and your app. What you run in a plain terminal stays you.
