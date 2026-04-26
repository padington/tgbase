# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-04-26

### Added
- Initial bot scaffold using `go-telegram-bot-api/v5`
- Long-polling update loop with graceful shutdown on SIGINT/SIGTERM
- `/ping` command — replies `pong`
- Multi-stage Dockerfile for production VPS deployment
- `.env.example` with `TELEGRAM_BOT_TOKEN` and `BOT_DEBUG` variables
