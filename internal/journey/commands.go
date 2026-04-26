package journey

import (
	"log"
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

// HandleAbout sends the bot description in the user's locale.
func (r *Runner) HandleAbout(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)
	text := r.trans.T("about", locale, nil)
	if _, err := s.Send(tgbotapi.NewMessage(msg.Chat.ID, text)); err != nil {
		log.Printf("journey: about send: %v", err)
	}
}

// HandleReport prints a per-user breakdown of completed / in-progress /
// not_tolerated / interrupted products.
func (r *Runner) HandleReport(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)

	if len(user.Products) == 0 {
		send(s, msg.Chat.ID, r.trans.T("cmd.report.empty", locale, nil))
		return
	}

	var completed, notTolerated, interrupted []string
	for _, name := range sortedKeys(user.Products) {
		prog := user.Products[name]
		switch prog.Status {
		case "completed":
			completed = append(completed, name)
		case "not_tolerated":
			notTolerated = append(notTolerated, name)
		case "interrupted":
			interrupted = append(interrupted, name)
		}
	}

	var lines []string
	lines = append(lines, r.trans.T("cmd.report.heading", locale, nil))
	if user.CurrentProduct != "" {
		lines = append(lines, r.trans.T("cmd.report.in_progress", locale, map[string]any{
			"name":  user.CurrentProduct,
			"stage": user.CurrentStage,
		}))
	}
	if len(completed) > 0 {
		lines = append(lines, r.trans.T("cmd.report.completed", locale, map[string]any{
			"names": strings.Join(completed, ", "),
		}))
	}
	if len(notTolerated) > 0 {
		lines = append(lines, r.trans.T("cmd.report.not_tolerated", locale, map[string]any{
			"names": strings.Join(notTolerated, ", "),
		}))
	}
	if len(interrupted) > 0 {
		lines = append(lines, r.trans.T("cmd.report.interrupted", locale, map[string]any{
			"names": strings.Join(interrupted, ", "),
		}))
	}
	send(s, msg.Chat.ID, strings.Join(lines, "\n"))
}

// HandleAbandon marks the active trial as interrupted and re-routes to
// the product selection phase.
func (r *Runner) HandleAbandon(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)

	if user.CurrentProduct == "" {
		send(s, msg.Chat.ID, r.trans.T("cmd.abandon.no_active", locale, nil))
		return
	}

	abandoned := user.CurrentProduct
	displayName := abandoned
	if prod, ok := r.catalog.Find(abandoned); ok {
		displayName = prod.DisplayName(locale)
	}

	if user.Products == nil {
		user.Products = make(map[string]state.ProductProgress)
	}
	prog := user.Products[abandoned]
	prog.Status = "interrupted"
	prog.LastStage = user.CurrentStage
	prog.UpdatedAt = r.now()
	user.Products[abandoned] = prog
	user.CurrentProduct = ""
	user.CurrentStage = ""
	user.OfferedProducts = nil
	user.State = state.StateAwaitingProductChoice
	r.store.Set(msg.From.ID, user)

	send(s, msg.Chat.ID, r.trans.T("cmd.abandon.confirmed", locale, map[string]any{
		"name": displayName,
	}))

	if phase, ok := r.phases[state.StateAwaitingProductChoice]; ok {
		ctx := r.contextFor(msg.From.ID, user)
		r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
	}
}

func (r *Runner) localeForUser(u state.UserData) i18n.Locale {
	if u.Locale != "" {
		return i18n.Locale(u.Locale)
	}
	return i18n.Locale(r.settings.Get().DefaultLocale)
}

func send(s router.Sender, chatID int64, text string) {
	if _, err := s.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		log.Printf("journey: send: %v", err)
	}
}

func sortedKeys(m map[string]state.ProductProgress) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
