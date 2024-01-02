package cmd

import (
	"context"
	"time"

	"captcha-lite/captcha"
	"captcha-lite/locale"
	"captcha-lite/logger"
	"captcha-lite/underattack"

	"github.com/allegro/bigcache/v3"
	tb "gopkg.in/telebot.v3"
)

// Dependency contains the dependency injection struct
// that is required for the main command to use.
//
// It will spread and use the correct dependencies for
// each packages on the captcha project.
type Dependency struct {
	Memory   *bigcache.BigCache
	Bot      *tb.Bot
	Logger   logger.Logger
	Language string
	captcha  *captcha.Dependencies

	UnderAttack *underattack.Dependency
}

// New returns a pointer struct of Dependency
// which map the incoming dependencies provided
// into what's needed by each domain.
func New(deps Dependency) *Dependency {
	var localeLanguage map[locale.Message]string
	switch deps.Language {
	case "id":
		localeLanguage = locale.ID
	default:
		localeLanguage = locale.EN
	}

	var underAttackDependency *underattack.Dependency = nil
	if deps.UnderAttack != nil {
		underAttackDependency = &underattack.Dependency{
			Datastore: deps.UnderAttack.Datastore,
			Memory:    deps.Memory,
			Bot:       deps.Bot,
			Logger:    deps.Logger,
			Locale:    localeLanguage,
		}
	}
	return &Dependency{
		captcha: &captcha.Dependencies{
			Memory: deps.Memory,
			Bot:    deps.Bot,
			Locale: localeLanguage,
			Log:    deps.Logger,
		},
		UnderAttack: underAttackDependency,
	}
}

// OnTextHandler handle any incoming text from the group
func (d *Dependency) OnTextHandler(c tb.Context) error {
	d.captcha.WaitForAnswer(c.Message())
	return nil
}

// OnUserJoinHandler handle any incoming user join,
// whether they were invited by someone (meaning they are
// added by someone else into the group), or they join
// the group all by themselves.
func (d *Dependency) OnUserJoinHandler(c tb.Context) error {
	if d.UnderAttack != nil {
		// This block will be executed only if the UnderAttack struct is not nil
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()

		underAttack, err := d.UnderAttack.AreWe(ctx, c.Chat().ID)
		if err != nil {
			d.Logger.HandleError(err)
		}

		if underAttack {
			err := c.Bot().Ban(c.Chat(), &tb.ChatMember{User: c.Sender(), RestrictedUntil: tb.Forever()})
			if err != nil {
				d.Logger.HandleBotError(err, d.Bot, c.Message())
			}
			return nil
		}
	}

	d.captcha.CaptchaUserJoin(c.Message())
	return nil
}

// OnNonTextHandler meant to handle anything else
// than an incoming text message.
func (d *Dependency) OnNonTextHandler(c tb.Context) error {
	d.captcha.NonTextListener(c.Message())
	return nil
}

// OnUserLeftHandler handles during an event in which
// a user left the group.
func (d *Dependency) OnUserLeftHandler(c tb.Context) error {
	d.captcha.CaptchaUserLeave(c.Message())
	return nil
}
