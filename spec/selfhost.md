# Selfhosting the JitTrippin Daemon

The JitTrippin Daemon is just a REST API.
The best way, so far is to run it as a **systemd** service

## 1. Make a directory for the service

```sh
mkdir -p /opt/jittrippin
cd /opt/jittrippin
```

_**OR Clone the repo**_

```sh
git clone https://github.com/nxrmqlly/jittrippin.git /opt/jittrippin
cd /opt/jittrippin
```

## 2. Setup PostgreSQL

It's very simple to create a PostgreSQL database with Docker.

```sh
docker run \
    --name jittrippin-postgres \
    -e POSTGRES_USER=admin \
    -e POSTGRES_PASSWORD=jittrippin123 \
    -e POSTGRES_DB=jtdb \
    -p 5432:5432 \
    -d postgres:latest
```

This results in your `JTD_POSTGRES_CONNSTR` to look like:

```
postgres://admin:jittrippin123@localhost:5432/jtdb?sslmode=disable
```

You should use a more secure password rather than "jittrippin123"

Set it in the env.

## 3. Setup the Github App

Go to [GitHub Developer Settings](https://github.com/settings/apps/) and create a "New Github App"

> **OAuth App != GitHub App**
>
> JitTrippin uses a GitHub App for both GitHub OAuth and GitHub repository integration.

### Basic Configuration

Give it a name, for example "My JitTrippin" and configure it as the following:

1. **Homepage URL**: `JTD_PUBLIC_URL` from env OR anything else.
2. **Redirect URI**: Set it to exactly the same value as `JTD_REDIRECT_URL` from env.

> Example: `https://jt.example.com/api/v1/auth/github/callback`

3. **Expire user authorization tokens**: Enable this.

4. **Setup URL**: Set to `JTD_PUBLIC_URL` + `/api/v1/integrations/github/install-callback`

> Example: `https://jt.example.com/api/v1/integrations/github/install-callback`

5. **Webhook > Active**: Enable this.
6. **Webhook > Webhook URL**: set it to the `JTD_PUBLIC_URL` + `/api/v1/integrations/github/webhook`

> Example: `https://jt.example.com/api/v1/integrations/github/webhook`

7. **Webhook > Webhook Secret**: Set to the same value you will use for `GITHUB_WEBHOOK_SECRET` (see "Configuring the Environment" section)

### Repository permissions

1. **Actions**: Read-only
2. **Artifact metadata**: Read and write
3. **Commit statuses**: Read and write
4. **Contents**: Read and write
5. **Deployments**: Read and write
6. **Issues**: Read and write
7. **Pull requests**: Read and write
8. **Webhooks**: Read and write
9. **Workflows**: Read and write

### Account permissions:

1. **Email addresses**: Read-only

### Events:

Subscribe to these events used by JitTrippin:

1. Installation target
2. Meta
3. Delete
4. Issues
5. Public
6. Pull Request
7. Pull request review
8. Pull request review comment
9. Pull request review thread
10. Push
11. Release
12. Repository
13. Status

### Where can this GitHub App be installed?

Choose Only on this account if you are running JitTrippin privately for yourself.

Choose Any account if you intend to let other GitHub users or organizations install your JitTrippin instance.

Once the configuration is complete, click **Create GitHub App.**

## Github App ID, ClientID and App Slug

1. Under the "About" section, copy the "App ID". This is the value for `GITHUB_APP_ID` in env.
2. Copy the "Client ID" in the same section. This is the value for `GITHUB_CLIENT_ID` in env.
3. Look at your browser URL bar, it should be something like `https://github.com/settings/apps/my-jittrippin`. `my-jittrippin` is the value for `GITHUB_APP_SLUG`

## 4. GitHub App Private Key and Client Secret

### Client Secret

1. Under "Client secrets" Generate a new Client secret.
2. Save it as you can only see it once.
3. This is the value for `GITHUB_CLIENT_SECRET` in env.

### Private Key

1. Scroll down and find the **Private keys** section and generate a private key.
2. GitHub will download a .pem file. _**KEEP THIS FILE PRIVATE**_
3. Save it at the repository's root (you can name it `github-app.pem` and it will get .gitignore'd automatically)
4. Depending on where you saved this file, or if you have saved it as `github-app.pem` in the repo root, you can set `GITHUB_PRIVATE_KEY_PATH` in env.

## 5. Configuring the Environment

1. cd into the directory where you cloned the repo.
2. `mv example.env .env`

### Generate Secrets

You should use unpredictable random values for both `JTD_SIGNING_SECRET` and `GITHUB_WEBHOOK_SECRET`.

A simple way to generate one is:

```sh
openssl rand -hex 32
```

Run it twice, once for each secret:

```sh
echo "JTD_SIGNING_SECRET=$(openssl rand -hex 32)"
echo "GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 32)"
```

This generates 32 random bytes (256 bits) and represents them as hexadecimal text.
**Do not reuse the same value for both secrets.**

### Populate `.env`

Fill in the rest of .env

By this point you should have every value needed from the previous steps:

- your PostgreSQL connection string (step 2)
- your GitHub App's ID, Client ID, Client Secret, App Slug, and private key path (steps 3-4)

#### Put it all together:

```sh
# --- Database ---
JTD_POSTGRES_CONNSTR=postgres://admin:jittrippin123@localhost:5432/jtdb?sslmode=disable

# --- Core / Networking ---
JTD_SIGNING_SECRET=(generated above)
JTD_PUBLIC_URL=https://jt.example.com
JTD_REDIRECT_URL=https://jt.example.com/api/v1/auth/github/callback
JTD_BIND_ADDR=127.0.0.1:5500

# --- GitHub App ---
GITHUB_CLIENT_ID=(from GitHub App "About" section)
GITHUB_CLIENT_SECRET=(from Github App "Client secrets")
GITHUB_APP_ID=(from GitHub App "About" section)
GITHUB_PRIVATE_KEY_PATH=/opt/jittrippin/github-app.pem
GITHUB_WEBHOOK_SECRET=(generated above)
GITHUB_APP_SLUG=my-jittrippin
```

#### Notes:

1. `JTD_BIND_ADDR` is the local address `jtd` listens on; bind it to `127.0.0.1:5500` (IPv4) and let your reverse proxy handle public HTTPS
2. `GITHUB_PRIVATE_KEY_PATH` should correctly point to the `.pem` file you downloaded from GitHub
3. `JTD_PUBLIC_URL` and `JTD_REDIRECT_URL` must exactly match what you entered in the fields in step 3

## 6. Install `jtd`

### Option A: Install the daemon:

```sh
go install github.com/nxrmqlly/jittrippin/cmd/jtd@latest
```

Copy the resulting binary somewhere systemd can execute it, for example:

```sh
sudo cp "$(go env GOPATH)/bin/jtd" /usr/local/bin/jtd
```

### Option B: Build from source

If you cloned the repository, you can build `jtd` directly:

```sh
go build -o /usr/local/bin/jtd ./cmd/jtd
```

## 7. Give `jtd` its own user + access to docker

JitTrippin has its own user and uses the host's Docker daemon to run pipeline containers.

Make a new user, give it ownership of its dir and add it to the docker group:

```sh
sudo useradd --system --home /opt/jittrippin --shell /usr/sbin/nologin jittrippin
sudo chown -R jittrippin:jittrippin /opt/jittrippin
sudo usermod -aG docker jittrippin
```

Verify:

```sh
sudo -u jittrippin docker ps
```

If this does not work, `jtd` will _not_ be able to execute pipeline jobs.

> The Docker group grants root-equivalent access to the host. Only give this access to a trusted service account.

## 8. Configure `systemd`

1. Create the service file

```sh
sudo touch /etc/systemd/system/jittrippin.service
```

And paste this in:

```ini
[Unit]
Description=JitTrippin Daemon
After=network-online.target docker.service postgresql.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
User=jittrippin
Group=jittrippin

WorkingDirectory=/opt/jittrippin
EnvironmentFile=/opt/jittrippin/.env

ExecStart=/usr/local/bin/jtd

Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Reload systemd and start JitTrippin:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now jittrippin
sudo systemctl status jittrippin
```

For logs:

```sh
sudo journalctl -u jittrippin -f
```

## 9. Configure your reverse proxy (if you have one)

Point your domain at the server and configure your reverse proxy or tunnel to forward HTTPS requests to `jtd`.

Your public URL must match the value configured in .env:

```env
JTD_PUBLIC_URL=https://jt.example.com
```

## 10. Install and use

Configure the CLI:

```sh
jt daemon --set https://jt.example.com # your public url here
jt auth login
```

Connect a repo:

```sh
jt repos add
```

And run a pipeline remotely:

```sh
jt run
```

## Finally, pat yourself on the back.
