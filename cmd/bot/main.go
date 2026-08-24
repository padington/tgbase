package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/padington/tgbase/internal/bot"
	"github.com/padington/tgbase/internal/config"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	appCfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	cfg := bot.Config{
		Token:              token,
		Debug:              os.Getenv("BOT_DEBUG") == "true",
		Timeout:            60,
		Env:                os.Getenv("BOT_ENV"),
		DataDir:            os.Getenv("DATA_DIR"),
		DataPath:           os.Getenv("DATA_PATH"),
		StateFlushInterval: time.Duration(appCfg.State.FlushInterval),
		ProductsSeedPath:   appCfg.Bootstrap.ProductsSeedPath,
		SettingsSeedPath:   appCfg.Bootstrap.SettingsSeedPath,
		I18nDir:            appCfg.Bootstrap.I18nDir,
		ScreeningDir:       appCfg.Bootstrap.ScreeningDir,
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
