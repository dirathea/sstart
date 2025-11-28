# 🤫 sstart: Secure Start for Cloud-Native Secrets
sstart is a minimalist, zero-persistence CLI tool that securely retrieves application secrets from multiple backend sources (Vault, AWS, GCP, Azure) and injects them as environment variables into any wrapped process.

It is the spiritual successor to the [Teller](https://github.com/tellerops/teller), modernized and rebuilt in Go for fast execution, reliability, and cross-platform simplicity.

## 🎯 The Problem sstart Solves
For local development, teams often choose between two bad options:

1. Static .env files: Highly insecure, prone to being committed to Git, and impossible to audit.

2. Custom scripts: Complex, unmaintainable shell scripts that only talk to one vault and are difficult to standardize across projects.

sstart eliminates both. You define all your required secrets from all your sources (e.g., database password from Vault, API key from AWS) in a single, declarative .sstart.yml file.

## Features

- 🔐 **Multiple Secret Providers**: Support for AWS Secrets Manager, dotenv files, and more
- 🔄 **Combine Secrets**: Merge secrets from multiple providers
- 🚀 **Subprocess Execution**: Automatically inject secrets into subprocesses
- 🔒 **Secure by Default**: Secrets never appear in shell history or logs
- ⚙️ **YAML Configuration**: Easy-to-use configuration file

## Installation

```bash
go install github.com/dirathea/sstart/cmd/sstart@latest
```

## Quick Start

1. Create a `.sstart.yml` configuration file:

```yaml
providers:
  - kind: aws_secretsmanager
    id: prod
    secret_id: myapp/production
    keys:
      API_KEY: ==
      DATABASE_URL: ==
  
  - kind: dotenv
    id: dev
    path: .env.local
```

2. Run a command with secrets injected:

```bash
sstart run -- node index.js
```

## Commands

### `sstart run`

Run a command with injected secrets:

```bash
sstart run -- node index.js
sstart run --reset --providers aws-prod,dotenv-dev -- python app.py
```

Flags:
- `--reset`: Reset environment variables before injecting secrets
- `--providers`: Comma-separated list of provider IDs to use (default: all providers)
- `--config, -c`: Path to configuration file (default: `.sstart.yml`)

### `sstart show`

Show collected secrets (masked for security):

```bash
sstart show
sstart show --providers aws-prod,dotenv-dev
```

Flags:
- `--providers`: Comma-separated list of provider IDs to use (default: all providers)

### `sstart env`

Export secrets in environment variable format:

```bash
# Shell format
sstart env

# JSON format
sstart env --format json

# YAML format
sstart env --format yaml

# Docker usage
docker run --env-file <(sstart env) alpine sh

# Use specific providers
sstart env --providers aws-prod,dotenv-dev
```

Flags:
- `--format`: Output format: `shell` (default), `json`, or `yaml`
- `--providers`: Comma-separated list of provider IDs to use (default: all providers)

### `sstart sh`

Generate shell commands to export secrets:

```bash
eval "$(sstart sh)"
source <(sstart sh)
```

Flags:
- `--providers`: Comma-separated list of provider IDs to use (default: all providers)

## Configuration

The `.sstart.yml` file defines your providers and secret mappings:

```yaml
providers:
  - kind: provider_kind
    id: provider_id  # Optional: defaults to 'kind'. Required if multiple providers share the same 'kind'
    path: path/to/secret
    keys:
      SOURCE_KEY: TARGET_KEY
      ANOTHER_KEY: ==  # == means keep same name
```

**Important**: Each provider loads from a single source. If you need to load multiple secrets from the same provider type (e.g., multiple paths from AWS Secrets Manager), configure multiple provider instances with the same `kind` but different `id` values. When multiple providers share the same `kind`, each must have an explicit, unique `id`.

### Provider Kinds

| Provider | Status |
|----------|--------|
| `aws_secretsmanager` | Stable |
| `dotenv` | Stable |
| `vault` | Stable |

### Template Variables

You can use template variables in paths:

```yaml
providers:
  - kind: aws_secretsmanager
    id: env
    secret_id: myapp/{{ get_env(name="ENVIRONMENT", default="development") }}
```

You can also use simple environment variable expansion with `${VAR}` or `$VAR` syntax:
```yaml
  - kind: dotenv
    id: shared
    path: ${HOME}/.config/myapp/.env
```

## Examples

### Using with Node.js

```bash
sstart run -- node index.js
```

### Using with Docker

```bash
docker run --rm -it --env-file <(sstart env) node:18-alpine sh
```

### Multiple Providers

Each provider loads from a single source. To load multiple secrets from the same provider type, create multiple provider instances:

```yaml
providers:
  # First AWS Secrets Manager provider - production
  - kind: aws_secretsmanager
    id: aws-prod  # Explicit ID required because there are multiple providers of this kind
    secret_id: myapp/production
  
  # Second AWS Secrets Manager provider - staging
  - kind: aws_secretsmanager
    id: aws-staging  # Explicit ID required because there are multiple providers of this kind
    secret_id: myapp/staging
  
  - kind: dotenv
    # ID not specified - defaults to 'dotenv'
    path: .env.local
  
  - kind: aws_secretsmanager
    id: shared-aws  # Explicit ID required because there are multiple providers of this kind
    secret_id: shared/secrets
```

When multiple providers share the same `kind`, each must have an explicit, unique `id`. Otherwise, `id` defaults to the `kind` value. Use the `id` values with the `--providers` flag:

```bash
sstart run --providers aws-prod,shared-aws -- node app.js
```

## Security

- Secrets are never logged or displayed in full
- Use `--reset` to ensure a clean environment
- Secrets are injected directly into subprocess environment, never exposed to shell
- Configuration files should be added to `.gitignore`

## License

Apache-2.0

