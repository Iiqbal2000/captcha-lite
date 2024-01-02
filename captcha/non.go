package captcha

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"captcha-lite/locale"
	"captcha-lite/utils"

	tb "gopkg.in/telebot.v3"
)

// NonTextListener is the handler for every incoming payload that
// is not a text format.
func (d *Dependencies) NonTextListener(m *tb.Message) {
	// Check if the message author is in the captcha:users list or not
	// If not, return
	// If yes, check if the answer is correct or not
	exists, err := userExists(d.Memory, strconv.FormatInt(m.Sender.ID, 10))
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	if !exists {
		return
	}

	// Check if the answer is correct or not.
	// If not, ask them to give the correct answer and time remaining.
	// If yes, delete the message and remove the user from the captcha:users list.
	//
	// Get the answer and all the data surrounding captcha from
	// this specific user ID from the cache.
	data, err := d.Memory.Get(strconv.FormatInt(m.Sender.ID, 10))
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	var captcha Captcha
	err = json.Unmarshal(data, &captcha)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	// Check if the answer is a media
	remainingTime := time.Until(captcha.Expiry)
	message := strings.NewReplacer(
		"{{user}}", "<a href=\"tg://user?id="+strconv.FormatInt(m.Sender.ID, 10)+"\">"+
			sanitizeInput(m.Sender.FirstName)+
			utils.ShouldAddSpace(m.Sender)+
			sanitizeInput(m.Sender.LastName)+
			"</a>. ",
		"{{remaining}}", strconv.Itoa(int(remainingTime.Seconds())),
	).Replace(d.Locale[locale.MessageNonText])
	wrongMsg, err := d.Bot.Send(
		m.Chat,
		message,
		&tb.SendOptions{
			ParseMode:             tb.ModeHTML,
			DisableWebPagePreview: true,
		},
	)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	err = d.Bot.Delete(m)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	err = d.collectAdditionalAndCache(&captcha, m, wrongMsg)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}
}
