# fleet

Sets up and checks the machines AI agents run on, from one Go program.

## The doctor

A read-only check of this machine. It detects the system (`runtime.GOOS`): a Mac is supported, other systems say "not
supported yet". It reads your settings from `fleet/settings.toml` in your system's config folder
(`os.UserConfigDir()`: `~/Library/Application Support` on a Mac), then checks Homebrew, git, gh, Go, Claude Code, T3
Code, that Tailscale is up and that the Mac never sleeps. Each line is a tick, or a cross with the fix; it exits 0 when
all pass.

```
go run ./cmd/fleet doctor
```

Each check is a function tested with sample output (`go test ./...`), on Linux in CI.
