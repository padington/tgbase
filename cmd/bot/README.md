# cmd/bot

Process entry point. Reads env vars and `config.yaml`, builds a `bot.Config`, calls `bot.New`, and runs the bot until SIGINT/SIGTERM.

## Responsibility

Wire env → config → `bot.Bot`. Nothing else lives here.

## Env vars consumed

| Variable | Required | Purpose |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | yes | passed to tgbotapi |
| `BOT_DEBUG` | no | "true" enables tgbotapi debug logging |
| `BOT_ENV` | no | label for `/whoami` |
| `DATA_DIR` | no | backend dir; empty → in-memory backend |
| `DATA_PATH` | no | legacy single-file path; superseded by `DATA_DIR` |
| `CONFIG_PATH` | no | YAML config; defaults to `config.yaml` |

## When to edit

- New env-var input → add to the `bot.Config` build here.
- New SIGINT/SIGTERM handling → here.
- Anything else: edit the package the change actually belongs to. This file is intentionally thin.
