# tgbase

A Go-based Telegram bot platform. Receives updates via long polling.

## Commands

| Command | Response |
|---------|----------|
| `/ping` | `pong`   |

## Prerequisites

- [Go 1.23+](https://go.dev/dl/) (for local development)
- [Docker](https://docs.docker.com/get-docker/) (for VPS deployment)
- A Telegram bot token from [@BotFather](https://t.me/BotFather)

## Local development

```bash
# 1. Clone and enter the repo
git clone https://github.com/padington/tgbase.git
cd tgbase

# 2. Install dependencies
go mod tidy

# 3. Set your token
cp .env.example .env
# edit .env — fill in TELEGRAM_BOT_TOKEN

# 4. Run
export $(cat .env | xargs)
go run ./cmd/bot
```

## Docker

```bash
# Build
docker build -t tgbase .

# Run
docker run -d \
  --name tgbot \
  --restart unless-stopped \
  -e TELEGRAM_BOT_TOKEN=your_token_here \
  tgbase
```

## VPS deployment

```bash
# On your local machine — build and push the image
docker build -t ghcr.io/padington/tgbase:latest .
docker push ghcr.io/padington/tgbase:latest

# SSH into the VPS, then:
docker pull ghcr.io/padington/tgbase:latest
docker run -d \
  --name tgbot \
  --restart unless-stopped \
  -e TELEGRAM_BOT_TOKEN=your_token_here \
  ghcr.io/padington/tgbase:latest
```

Alternatively, copy the repo to the VPS and build there:

```bash
scp -r . user@your-vps:/opt/tgbase
ssh user@your-vps "cd /opt/tgbase && docker build -t tgbase . && docker run -d --name tgbot --restart unless-stopped -e TELEGRAM_BOT_TOKEN=your_token_here tgbase"
```

## Environment variables

| Variable             | Required | Default | Description                  |
|----------------------|----------|---------|------------------------------|
| `TELEGRAM_BOT_TOKEN` | yes      | —       | Token from @BotFather        |
| `BOT_DEBUG`          | no       | `false` | Log every Telegram API call  |

## Project layout

```
cmd/bot/        — entry point
internal/bot/   — bot core (polling loop + handlers)
Dockerfile      — multi-stage production image
.env.example    — environment variable template
```
