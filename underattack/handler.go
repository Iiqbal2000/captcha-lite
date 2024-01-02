package underattack

import (
	"context"
	"strconv"
	"strings"
	"time"

	"captcha-lite/locale"
	"captcha-lite/utils"

	tb "gopkg.in/telebot.v3"
)

// EnableUnderAttackModeHandler provides a handler for /underattack command.
func (d *Dependency) EnableUnderAttackModeHandler(c tb.Context) error {
	if c.Message().Private() || c.Sender().IsBot {
		return nil
	}

	admins, err := c.Bot().AdminsOf(c.Chat())
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	if !utils.IsAdmin(admins, c.Sender()) {
		_, err := c.Bot().Send(
			c.Chat(),
			d.Locale[locale.MessageUnderAttackOnlyAdmin],
			&tb.SendOptions{
				ReplyTo:           c.Message(),
				AllowWithoutReply: true,
			},
		)
		if err != nil {
			d.Logger.HandleBotError(err, d.Bot, c.Message())
			return nil
		}

		return nil
	}

	// Sender must be an admin here.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	// Check if we are on the under attack mode right now.
	underAttackModeEnabled, err := d.AreWe(ctx, c.Chat().ID)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	if underAttackModeEnabled {
		_, err := c.Bot().Send(
			c.Chat(),
			d.Locale[locale.MessageUnderAttackAlreadyEnabled],
			&tb.SendOptions{
				ReplyTo:           c.Message(),
				AllowWithoutReply: true,
			},
		)
		if err != nil {
			d.Logger.HandleBotError(err, d.Bot, c.Message())
			return nil
		}

		return nil
	}

	expiresAt := time.Now().Add(time.Minute * 30)

	notificationMessage, err := c.Bot().Send(
		c.Chat(),
		strings.Replace(
			d.Locale[locale.MessageUnderAttackStarting],
			"{{expiresAt}}",
			expiresAt.In(time.UTC).Format("15:04 MST"),
			1,
		),
		&tb.SendOptions{
			ParseMode: tb.ModeDefault,
		},
	)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = d.Datastore.SetUnderAttackStatus(ctx, c.Chat().ID, true, time.Now().Add(time.Minute*30), int64(notificationMessage.ID))
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = d.Memory.Delete("underattack:" + strconv.FormatInt(c.Chat().ID, 10))
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = c.Bot().Pin(notificationMessage)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	return nil
}

// DisableUnderAttackModeHandler provides a handler for /disableunderattack command.
func (d *Dependency) DisableUnderAttackModeHandler(c tb.Context) error {
	if c.Message().Private() || c.Sender().IsBot {
		return nil
	}

	admins, err := c.Bot().AdminsOf(c.Chat())
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	if !utils.IsAdmin(admins, c.Sender()) {
		_, err := c.Bot().Send(
			c.Chat(),
			d.Locale[locale.MessageUnderAttackOnlyAdmin],
			&tb.SendOptions{
				ReplyTo:           c.Message(),
				AllowWithoutReply: true,
			},
		)
		if err != nil {
			d.Logger.HandleBotError(err, d.Bot, c.Message())
			return nil
		}

		return nil
	}

	// Sender must be an admin here.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	// Check if we are on the under attack mode right now.
	underAttackModeEnabled, err := d.AreWe(ctx, c.Chat().ID)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	if !underAttackModeEnabled {
		return nil
	}

	underAttackEntry, err := d.Datastore.GetUnderAttackEntry(ctx, c.Chat().ID)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = d.Datastore.SetUnderAttackStatus(ctx, c.Chat().ID, false, time.Now(), 0)
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = d.Memory.Delete("underattack:" + strconv.FormatInt(c.Chat().ID, 10))
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	err = c.Bot().Unpin(c.Chat(), int(underAttackEntry.NotificationMessageID))
	if err != nil {
		d.Logger.HandleBotError(err, d.Bot, c.Message())
		return nil
	}

	return nil
}
