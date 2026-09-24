# Deploy this source tree to the local Docker/Caddy stack

On the configured host, `make deploy` builds the current checkout and runs it
with PostgreSQL and Redis on a private Compose network. The app alone joins the
existing `nas-caddy` Docker network; it does not publish an application port on
the host. Caddy serves `https://api2.pinellia.uk`.

The first run creates `deploy/.env` with random database, JWT, TOTP, and admin
secrets (mode 0600), then prints the initial admin login once. Keep `.env`
private and backed up: it contains the credentials and encryption keys needed
to keep the deployment working across restarts. Existing `.env` values are
preserved on subsequent deploys.

Useful commands:

```sh
make deploy         # build and update containers from this checkout
make deploy-status  # show service health
make deploy-logs    # follow service logs
make deploy-down    # stop services; preserves named volumes and data
```

## Switch application images without starting a second service

The image that was running before the multi-group upgrade is preserved locally as
`sub2api-local:backup-before-group-routing`. After the new image is built, only
one `sub2api` container runs at a time:

```sh
make deploy-backup   # switch the app to the preserved image (no rebuild)
make deploy-current  # switch the app back to sub2api-local:latest (no rebuild)
```

These commands do not restart PostgreSQL/Redis, clear volumes, or run a Docker
build. The inactive version is stored as an image, **not** a second running
service. The application's automatic database migrations are not rolled back
by switching images; check old-version compatibility before a longer rollback.
Do not remove the backup image until it is no longer needed.

This deployment expects Docker Compose, `openssl`, Python 3, the existing
external Docker network `nas-caddy`, and a Caddy container configured to route
`api2.pinellia.uk` to `sub2api:8080`. The host Caddyfile and TLS certificate
state live outside this repository at `/home/minium/minium/Docker/caddy/`.
