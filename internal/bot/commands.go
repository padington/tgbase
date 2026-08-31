package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
)

// commandRegistrar is the narrow slice of *tgbotapi.BotAPI needed to register
// the command menu; kept as an interface so tests can capture the calls.
type commandRegistrar interface {
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// modes says which optional tracks this binary actually wired. The menu is
// built from it so the client hint can never advertise a command the router
// does not handle.
type modes struct {
	screening bool // ADHD track: /adhd, /adhd_delete
	mood      bool // mood module: /mood, /mood_delete
	eating    bool // eating track: /food, /food_delete
	pushups   bool // pushup track: /pushups, /pushups_delete
}

// menuCommands returns the client command menu in usage-frequency order.
// Descriptions come from i18n ("cmd.<name>.desc"). Deliberate omissions:
// /start (Telegram renders its own Start button) and the operator commands
// /ping and /whoami. Per-track entries appear only when that track is
// actually wired, so the menu always matches the running binary.
func menuCommands(trans i18n.Translator, locale i18n.Locale, m modes) []tgbotapi.BotCommand {
	entries := []struct {
		name    string
		enabled bool
	}{
		{"menu", true},
		{"adhd", m.screening},
		{"mood", m.mood},
		{"food", m.eating},
		{"pushups", m.pushups},
		{"report", true},
		{"about", true},
		{"abandon", true},
		{"adhd_delete", m.screening},
		{"mood_delete", m.mood},
		{"food_delete", m.eating},
		{"pushups_delete", m.pushups},
	}
	var out []tgbotapi.BotCommand
	for _, e := range entries {
		if !e.enabled {
			continue
		}
		out = append(out, tgbotapi.BotCommand{
			Command:     e.name,
			Description: trans.T("cmd."+e.name+".desc", locale, nil),
		})
	}
	return out
}

// menuConfigs builds the setMyCommands payloads: the default-scope list with
// default-locale (ru) descriptions, plus a language_code="en" variant that
// overrides it for clients running in English.
func menuConfigs(trans i18n.Translator, m modes) []tgbotapi.SetMyCommandsConfig {
	return []tgbotapi.SetMyCommandsConfig{
		tgbotapi.NewSetMyCommands(menuCommands(trans, "", m)...),
		tgbotapi.NewSetMyCommandsWithScopeAndLanguage(
			tgbotapi.NewBotCommandScopeDefault(), "en",
			menuCommands(trans, "en", m)...),
	}
}

// registerCommands self-registers the command menu on boot so the client
// hint never drifts from the deployed binary (no manual BotFather step).
// Failures are logged as warnings and never fatal: a broken setMyCommands
// call must not stop the bot from serving updates.
func registerCommands(api commandRegistrar, trans i18n.Translator, m modes) {
	for _, cfg := range menuConfigs(trans, m) {
		if _, err := api.Request(cfg); err != nil {
			log.Printf("bot: warning: setMyCommands (lang=%q) failed, command menu may be stale: %v", cfg.LanguageCode, err)
		}
	}
}
