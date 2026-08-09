# AGENTS.md

## Scope

This repository maintains CDT Guard, a small Python daemon that checks Alibaba Cloud CDT traffic and controls one configured ECS instance when the traffic threshold is crossed.

The active product supports exactly one Alibaba Cloud account and one ECS instance per process. Do not add multi-account or multi-instance configuration.

The active product lives on `master`. The retired Go + React console is preserved on `archive/fullstack-console` and must not be merged back into the active product unless explicitly requested.

## Project Boundaries

Keep the active project focused on one operational loop:

1. Load configuration from environment variables.
2. Query current CDT traffic.
3. Query the configured ECS instance state.
4. Decide whether an ECS start or stop action is required.
5. Execute the action and write clear logs.
6. Report the cycle result to one optional Uptime Kuma Push monitor.
7. Expose successful cycle health through the container healthcheck.

Current stack:

- Python 3.12
- Alibaba Cloud Python SDK
- Docker and Docker Compose
- GitHub Actions publishing to GHCR

The following are non-goals unless a new requirement explicitly justifies them:

- Web UI or HTTP API
- database persistence
- account management
- multi-cloud abstractions
- notification frameworks beyond the single Uptime Kuma Push integration
- workflow engines
- remote administration

## Repository Structure

```text
.github/workflows/docker-image.yml  # amd64 GHCR build
scripts/cdt_guard.py                # Guard process and healthcheck
tests/test_cdt_guard.py             # Unit tests with fake cloud and HTTP clients
Dockerfile                          # Runtime image
compose.yaml                        # Deployment example
requirements.txt                    # Pinned Python dependencies
README.md                           # User-facing configuration and operation
```

Do not create additional layers or directories unless they remove real complexity. Small, cohesive helpers may remain in `scripts/cdt_guard.py`; add modules only when the script has clear independent responsibilities that need separate tests.

## Development Rules

- Preserve existing environment variable names and image behavior unless a change is explicitly documented as breaking.
- Prefer the Python standard library and existing SDKs over new dependencies.
- Keep configuration parsing explicit and fail startup on invalid required values.
- Treat Alibaba Cloud responses as untrusted external input and validate fields before making resource decisions.
- Never log or commit AccessKey secrets, tokens, `.env` files, or real instance credentials.
- Keep start and stop actions idempotent by checking instance state first.
- Send at most one Uptime Kuma Push per cycle and use only `UPTIME_KUMA_PUSH_URL`.
- Let API and configuration failures surface in logs; do not fabricate successful checks or actions.
- Tests and local development must not call real Alibaba Cloud APIs unless the task explicitly supplies dedicated test credentials.
- Keep `ghcr.io/qqqasdwx/cdt-monitor:guard` compatible because it is the deployed rolling tag.

## Validation

Run the checks relevant to the change:

```bash
python -m compileall -q scripts
docker build -t cdt-monitor:guard-local .
docker compose -f compose.yaml config
```

When tests are present, run them before the image build. For workflow changes, verify the rendered YAML and ensure only `master` publishes the rolling `guard` tag.

Before committing, inspect the diff for credentials, unrelated generated files, stale full-stack references, and undocumented behavior changes.
