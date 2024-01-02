package captcha

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"captcha-lite/locale"
	"captcha-lite/utils"

	tb "gopkg.in/telebot.v3"
)

// Captcha struct keep all the data needed for the captcha
// for a certain user.
//
// It will be converted to JSON format (as array of bytes or []byte)
// and then will be stored to the in memory cache, with the key
// of the corresponding Telegram User ID.
type Captcha struct {
	// Store the correct answer for the captcha
	Answer string `json:"answer"`
	// Expiry time for the captcha
	Expiry             time.Time `json:"expiry"`
	ChatID             int64     `json:"chat_id"`
	QuestionID         string    `json:"question_id"`
	AdditionalMessages []string  `json:"additional_messages"`
	UserMessages       []string  `json:"user_messages"`
}

const (
	// BanDuration specifies how long a user will be banned in the group.
	BanDuration = 1 * time.Minute
	// Timeout specifies how long the captcha question will be valid.
	// After this time, the user will be kicked.
	// Or banned exactly, for one hour.
	Timeout = 1 * time.Minute
)

// CaptchaUserJoin is the most frustrating function that I've written
// at this point of time.
//
// As the function name says, it will prompt a captcha to the incoming user that
// has just joined the group.
//
// At the end of the function, it will create 2 goroutines in which
// both of them are responsible for kicking the user out of the group.
func (d *Dependencies) CaptchaUserJoin(m *tb.Message) {
	// Check if the user is an admin or bot first.
	// If they are, return.
	// If they're not, continue to execute the captcha.
	admins, err := d.Bot.AdminsOf(m.Chat)
	if err != nil {
		if !strings.Contains(err.Error(), "Gateway Timeout (504)") && !strings.Contains(err.Error(), "retry after") {
			d.Log.HandleBotError(err, d.Bot, m)
			return
		}
	}

	if m.UserJoined.ID != 0 {
		m.Sender = m.UserJoined
	}

	if m.Sender.IsBot || m.Private() || utils.IsAdmin(admins, m.Sender) {
		return
	}

	// randNum generates a random number (3 digit) in string format
	var randNum = utils.GenerateRandomNumber()
	// captcha generates ascii art from the randNum value
	var captcha = utils.GenerateAscii(randNum)

	// Replacing the template from CaptchaQuestion
	question := strings.NewReplacer(
		"{{captcha}}", captcha,
		"{{user}}", "<a href=\"tg://user?id="+strconv.FormatInt(m.Sender.ID, 10)+"\">"+
			sanitizeInput(m.Sender.FirstName)+utils.ShouldAddSpace(m.Sender)+sanitizeInput(m.Sender.LastName)+
			"</a>",
	).Replace(d.Locale[locale.MessageJoin])

SENDMSG_RETRY:
	// Send the question first.
	msgQuestion, err := d.Bot.Send(
		m.Chat,
		question,
		&tb.SendOptions{
			ParseMode:             tb.ModeHTML,
			ReplyTo:               m,
			DisableWebPagePreview: true,
		},
	)
	if err != nil {
		if strings.Contains(err.Error(), "retry after") {
			// Acquire the retry number
			retry, err := strconv.Atoi(strings.Split(strings.Split(err.Error(), "telegram: retry after ")[1], " ")[0])
			if err != nil {
				// If there's an error, we'll just retry after 10 second
				retry = 10
			}

			// Let's wait a bit and retry
			time.Sleep(time.Second * time.Duration(retry))
			goto SENDMSG_RETRY
		}

		if strings.Contains(err.Error(), "Gateway Timeout (504)") {
			time.Sleep(time.Second * 10)
			goto SENDMSG_RETRY
		}

		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	// OK. We've sent the question. Now we are going to prepare the data that will
	// be kept on the in-memory store.
	//
	// The AdditionalMessages key will be added later when there is an additional message
	// sent by the bot.
	captchaData, err := json.Marshal(Captcha{
		Expiry:     time.Now().Add(Timeout),
		ChatID:     m.Chat.ID,
		Answer:     randNum,
		QuestionID: strconv.Itoa(msgQuestion.ID),
	})
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	// Yes, the cache key is their User ID in string format.
	err = d.Memory.Set(strconv.FormatInt(m.Sender.ID, 10), captchaData)
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	err = d.Memory.Append("captcha:users", []byte(";"+strconv.FormatInt(m.Sender.ID, 10)))
	if err != nil {
		d.Log.HandleBotError(err, d.Bot, m)
		return
	}

	cond := sync.NewCond(&sync.Mutex{})
	go d.waitOrDelete(m, cond)
}

func sanitizeInput(inp string) string {
	return html.EscapeString(inp)
}
