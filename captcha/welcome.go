package captcha

import (
	"strconv"
	"strings"

	"captcha-lite/locale"
	"captcha-lite/utils"

	tb "gopkg.in/telebot.v3"
)

// sendWelcomeMessage literally does what it's written.
func (d *Dependencies) sendWelcomeMessage(m *tb.Message) error {
	msg, err := d.Bot.Send(
		m.Chat,
		strings.NewReplacer(
			"{{user}}",
			"<a href=\"tg://user?id="+strconv.FormatInt(m.Sender.ID, 10)+"\">"+
				sanitizeInput(m.Sender.FirstName)+utils.ShouldAddSpace(m.Sender)+sanitizeInput(m.Sender.LastName)+
				"</a>",
			"{{group}}", m.Chat.FirstName,
		).Replace(d.Locale[locale.MessageWelcome]),
		&tb.SendOptions{
			ReplyTo:               m,
			ParseMode:             tb.ModeHTML,
			DisableWebPagePreview: true,
			DisableNotification:   false,
			AllowWithoutReply:     true,
		},
	)
	if err != nil {
		return err
	}

	go d.deleteMessage(
		&tb.StoredMessage{MessageID: strconv.Itoa(msg.ID), ChatID: m.Chat.ID},
	)
	return nil
}
