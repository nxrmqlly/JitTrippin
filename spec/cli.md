# CLI

## Auth and integrations

1. Authenticate the CLI to a particular JitTrippin daemon.
2. GitHub App authentication is then a daemon capability/integration.

## JitTrippin config:

- `.jtrc`: project level (toml)
- `user-config.toml`: user level, for cli - stores daemon instance in use
- `jtd-config.toml`: server level, for jtd - DK if we need it yet

## CLI:

- All commands that require login and/or config must prompt user for that
- All commands that _may_ need login and/or config advertise warning

### Interactivity

Let's use charmbracelet/huh/v2
Example:

```
✓ GitHub account connected
✓ JitTrippin installed

? Repository
  > ritam.in4k/jittrippin
    ritam.in4k/arcfile
    ritam.in4k/foo

? Branch
  > main
    develop

✓ Pipeline configured
```

### Commmands:

- `jt auth`: authenticate with jtd daemon
    - `jt auth login`: logs into a jtd daemon (defined at userconfig)
        - OAuth callback
        - asks for daemon url (if ran first time) (default for now: jt.ritam.cc)
        - check if daemon even exists and is valid jtd (health 200 OK)
        - uses huh to advertise available oauth providers (asks the daemon for capabilities)
        - just like `gh auth login`
        - stores session on keyring or local file fallback

    - `jt auth logout`: logs out and server revokes said session
    - `jt auth status`: lists login status and integrations

- `jt run [--local] [--pipeline/-p <pipeline>]`: run all pipelines in .jtrc/pipelines.dir or one
    - if logged in: run remotely by default unless `--local` is passed
    - if not logged in: run locally and advertise a warning too
    - shows a checklist of pipelines and their jobs
        - shows 2-3 lines of logs at a time
        - on fail: advertise links to logs or stdout them
        - on other errs, show the err

- `jt init`: setup a .jtrc file in the repo
    - ask for proj name (default: dirname)
    - ask for dir (default: .jt/)

- `jt daemon [--set <daemon_url>]`: show or set daemon url
    - if show: calls the daemon health for 200 OK
    - if set: check if logged out:
        - if not logged out: prompt for logout
        - if logged out: call health for 200 OK and then set

- `jt repos`: list tracked repos (this and subcommands need login)
    - `jt repos add`: view connectable repos (ask daemon) and add
        - if only one provider, list repos otherwise:
        - ask for provider (ask daemon)
        - check if user has repo connection (ask the daemon)
        - ask for repo fullname (list select, ask daemon)
        - ask for branch (list select, ask daemon)

    - `jt repos remove`
        - ask daemon for tracked
        - untrack from select list

- `jt ping`: ask daemon health for 200 OK

- `jt integrations`: control user integrations on the jtd server (needs login)
    - `jt integrations add`: interactively add from daemon
    - `jt integrations remove`: interactively remove from daemon
    - `jt integrations`: list.

## Future

Extend `jt daemon` to allow for multiple jtd instances
