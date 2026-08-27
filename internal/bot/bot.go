package bot

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/flows/meta"
	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/reminder"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
)

type Config struct {
	Token   string
	Debug   bool
	Timeout int
	Env     string

	// DataDir is the directory holding the backend's per-key JSON files.
	// If empty, DataPath's parent dir is used; if both are empty, the bot
	// runs with an in-memory backend.
	DataDir            string
	StateFlushInterval time.Duration
	ProductsSeedPath   string
	SettingsSeedPath   string
	I18nDir            string

	// ScreeningDir holds the read-only self-check content YAMLs — ADHD,
	// mood and eating track (reloaded on every boot, never backend-seeded).
	// Empty disables every self-check track: /start keeps its legacy
	// behavior and the /adhd, /mood and /food commands are not registered.
	ScreeningDir string

	// DataPath is the legacy single-file persistence path. Kept so main.go
	// continues compiling while it transitions to DataDir.
	DataPath string
	Reminder reminder.Config // legacy, no longer consulted
}

type Bot struct {
	api      *tgbotapi.BotAPI
	cfg      Config
	router   *router.Router
	backend  store.Backend
	state    *state.Store
	runner   *journey.Runner
	reminder *reminder.Worker
}

func New(cfg Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorised as @%s", api.Self.UserName)

	backend, err := buildBackend(cfg)
	if err != nil {
		return nil, err
	}

	trans, err := i18n.Load(cfg.I18nDir, "ru")
	if err != nil {
		return nil, fmt.Errorf("load i18n from %s: %w", cfg.I18nDir, err)
	}

	catalog, err := products.New(backend, cfg.ProductsSeedPath)
	if err != nil {
		return nil, fmt.Errorf("load products: %w", err)
	}

	settingsStore, err := settings.New(backend, cfg.SettingsSeedPath)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	stateStore := state.NewStoreFromBackend(backend)

	var scrContent *screening.Content
	var moodContent *screening.MoodContent
	var eatingContent *screening.EatingContent
	if cfg.ScreeningDir != "" {
		scrContent, err = screening.Load(cfg.ScreeningDir)
		if err != nil {
			// Fail fast: running with wrong instrument texts or thresholds
			// is worse than not starting.
			return nil, fmt.Errorf("load screening content from %s: %w", cfg.ScreeningDir, err)
		}
		moodContent, err = screening.LoadMood(cfg.ScreeningDir)
		if err != nil {
			// Same fail-fast canon — and the crisis-card contacts are part
			// of the validated shape.
			return nil, fmt.Errorf("load mood content from %s: %w", cfg.ScreeningDir, err)
		}
		eatingContent, err = screening.LoadEating(cfg.ScreeningDir)
		if err != nil {
			// Same fail-fast canon: EDE-QS/BES/NIAS items and cutoffs are
			// pinned by the validator.
			return nil, fmt.Errorf("load eating content from %s: %w", cfg.ScreeningDir, err)
		}
	}

	runner := journey.New(stateStore, api, catalog, settingsStore, trans)
	for _, p := range phasesFor(scrContent, moodContent, eatingContent) {
		runner.Register(p)
	}

	worker := reminder.NewWithCallback(
		runner.Remind,
		func() time.Duration { return settingsStore.Get().ScanInterval },
	)

	r := router.New(api)

	r.HandleCommand("ping", meta.Ping())
	r.HandleCommand("whoami", meta.Whoami(cfg.Env))
	r.HandleText(exactText("Ping"), meta.Ping())
	r.HandleText(exactText("Whoami"), meta.Whoami(cfg.Env))

	r.HandleCommand("start", runner.HandleStart)
	// /menu is an alias of /start: both land on the home landing from any
	// state (legacy direct-to-diary start when the fork is unregistered).
	r.HandleCommand("menu", runner.HandleStart)
	r.HandleCommand("about", runner.HandleAbout)
	r.HandleCommand("report", runner.HandleReport)
	r.HandleCommand("abandon", runner.HandleAbandon)
	if scrContent != nil {
		r.HandleCommand("adhd", runner.HandleAdhd)
		r.HandleCommand("adhd_delete", runner.HandleAdhdDelete)
	}
	if moodContent != nil {
		r.HandleCommand("mood", runner.HandleMood)
		r.HandleCommand("mood_delete", runner.HandleMoodDelete)
	}
	if eatingContent != nil {
		r.HandleCommand("food", runner.HandleFood)
		r.HandleCommand("food_delete", runner.HandleFoodDelete)
	}

	r.HandleText(func(msg *tgbotapi.Message) bool {
		if msg.From == nil {
			return false
		}
		return runner.IsJourneyState(msg.From.ID)
	}, runner.HandleText)

	// Self-register the client command menu (setMyCommands) so the hint in
	// Telegram always matches what this binary actually handles. Best-effort:
	// a failure is a warning, not a boot error.
	registerCommands(api, trans, modes{
		screening: scrContent != nil,
		mood:      moodContent != nil,
		eating:    eatingContent != nil,
	})

	return &Bot{
		api:      api,
		cfg:      cfg,
		router:   r,
		backend:  backend,
		state:    stateStore,
		runner:   runner,
		reminder: worker,
	}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	defer func() {
		if err := b.backend.Close(); err != nil {
			log.Printf("bot: backend close: %v", err)
		}
	}()

	go b.reminder.Run(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.cfg.Timeout

	updates := b.api.GetUpdatesChan(u)
	log.Printf("polling for updates (timeout=%ds)", b.cfg.Timeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received, stopping updates")
			b.api.StopReceivingUpdates()
			return nil
		case update := <-updates:
			log.Printf("update id=%d", update.UpdateID)
			b.router.Dispatch(update)
		}
	}
}

func buildBackend(cfg Config) (store.Backend, error) {
	dir := cfg.DataDir
	if dir == "" && cfg.DataPath != "" {
		dir = filepath.Dir(cfg.DataPath)
	}
	if dir == "" {
		log.Println("bot: no DataDir/DataPath configured, using in-memory backend")
		return store.NewMemoryBackend(), nil
	}
	file, err := store.NewFileBackend(dir)
	if err != nil {
		return nil, fmt.Errorf("file backend: %w", err)
	}
	if cfg.StateFlushInterval <= 0 {
		log.Printf("bot: persisting to %s (synchronous)", dir)
		return file, nil
	}
	log.Printf("bot: persisting to %s (flush interval %s)", dir, cfg.StateFlushInterval)
	return store.NewDebouncedBackend(file, cfg.StateFlushInterval), nil
}

func exactText(s string) router.TextPredicate {
	return func(msg *tgbotapi.Message) bool { return msg.Text == s }
}
