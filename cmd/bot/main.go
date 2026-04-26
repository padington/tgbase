package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/padington/tgbase/internal/bot"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	cfg := bot.Config{
		Token:   token,
		Debug:   os.Getenv("BOT_DEBUG") == "true",
		Timeout: 60,
		Env:     os.Getenv("BOT_ENV"),
	}

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("init bot: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("bot started, press Ctrl+C to stop")
	if err := b.Run(ctx); err != nil {
		log.Fatalf("bot run: %v", err)
	}
	log.Println("bot stopped")
}
