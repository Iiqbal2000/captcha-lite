package captcha

import (
	"encoding/json"
	"strconv"

	"captcha-lite/utils"

	tb "gopkg.in/telebot.v3"
)

// CaptchaUserLeave handles the event when a user left the group.
// This will check if the user is in the memory of current active
// captcha or not.
//
// If it is, the captcha will be deleted.
func (d *Dependencies) CaptchaUserLeave(m *tb.Message) {
	// Check if the user is an admin or bot first.
	// If they are, return.
	// If they're not, continue to execute the captcha.
	admins, err := d.Bot.AdminsOf(m.Chat)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	if m.Sender.IsBot || m.Private() || utils.IsAdmin(admins, m.Sender) {
		return
	}

	// We need to check if the user is in the captcha:users cache
	// or not.
	check, err := userExists(d.Memory, strconv.FormatInt(m.Sender.ID, 10))
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	if !check {
		return
	}

	// OK, they exist in the cache. Now we've got to delete
	// all the message that we've sent before.
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

	err = d.removeUserFromCache(strconv.FormatInt(m.Sender.ID, 10))
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	// Delete the question message.
	err = d.Bot.Delete(&tb.StoredMessage{
		ChatID:    m.Chat.ID,
		MessageID: captcha.QuestionID,
	})
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	// Delete user's messages.
	for _, msgID := range captcha.UserMessages {
		if msgID == "" {
			continue
		}
		err = d.Bot.Delete(&tb.StoredMessage{
			ChatID:    m.Chat.ID,
			MessageID: msgID,
		})
		if err != nil {
			d.Log.HandleBotError(err, d.Bot, m)
			return
		}
	}

	// Delete any additional message.
	for _, msgID := range captcha.AdditionalMessages {
		if msgID == "" {
			continue
		}
		err = d.Bot.Delete(&tb.StoredMessage{
			ChatID:    m.Chat.ID,
			MessageID: msgID,
		})
		if err != nil {
			d.Log.HandleBotError(err, d.Bot, m)
			return
		}
	}
}
