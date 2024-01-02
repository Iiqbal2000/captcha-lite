// Hello!
//
// This is the source code for @TeknumCaptchaBot where you can find
// the ugly code behind @TeknumCaptchaBot's captcha feature and more.
//
// If you are learning Go for the first time and about to browse this
// repository as one of your first steps, you might want to read the
// other repository on the organization. It's far easier.
// Here: https://github.com/teknologi-umum/polarite
//
// Unless, you're stubborn and want to learn the hard way, all I can
// say is just... good luck.
//
// This source code is very ugly. Let me tell you that up front.
package main

import (
	"context"
	"database/sql"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	// Internals
	"captcha-lite/cmd"
	"captcha-lite/logger"
	"captcha-lite/logger/noop"
	rollbarlogger "captcha-lite/logger/rollbar"
	sentrylogger "captcha-lite/logger/sentry"
	zerologlogger "captcha-lite/logger/zerolog"
	"captcha-lite/underattack"
	"captcha-lite/underattack/datastore/memory"
	"captcha-lite/underattack/datastore/mysql"
	"captcha-lite/underattack/datastore/postgres"

	// Database and cache
	"github.com/allegro/bigcache/v3"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/rollbar/rollbar-go"

	// Others third party stuff
	"github.com/getsentry/sentry-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	tb "gopkg.in/telebot.v3"
)

// This init function checks if there's any configuration
// missing from the .env file.
func init() {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		log.Fatal("Please provide the ENVIRONMENT value on the .env file")
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("Please provide the BOT_TOKEN value on the .env file")
	}

	if os.Getenv("TZ") == "" {
		err := os.Setenv("TZ", "UTC")
		if err != nil {
			log.Fatalln("during setting TZ environment variable:", err)
		}
	}

	log.Println("Passed the environment variable check")
}

func main() {
	// Feature flags
	experimentalUnderAttack := flag.Bool("experimental-underattack", false, "Enable the experimental under attack module")

	// Setup in memory cache
	cache, err := bigcache.New(context.Background(), bigcache.Config{
		Shards:             1024,
		LifeWindow:         time.Minute * 5,
		CleanWindow:        time.Minute * 1,
		Verbose:            true,
		HardMaxCacheSize:   1024 * 1024 * 1024,
		MaxEntrySize:       500,
		MaxEntriesInWindow: 50,
	})
	if err != nil {
		log.Fatal("during creating a in memory cache:", errors.WithStack(err))
	}
	defer func(cache *bigcache.BigCache) {
		err := cache.Close()
		if err != nil {
			log.Fatal(errors.WithStack(err))
		}
	}(cache)

	// Setup logger client
	var loggerClient logger.Logger

	logProvider, ok := os.LookupEnv("LOG_PROVIDER")
	if !ok {
		logProvider = "noop"
	}

	switch strings.ToLower(logProvider) {
	case "noop":
		loggerClient = noop.New()
	case "sentry":
		// Setup Sentry for error handling.
		sentryClient, err := sentry.NewClient(sentry.ClientOptions{
			Dsn:              os.Getenv("SENTRY_DSN"),
			AttachStacktrace: true,
			Debug:            os.Getenv("ENVIRONMENT") == "development",
			Environment:      os.Getenv("ENVIRONMENT"),
		})
		if err != nil {
			log.Fatal("during initiating a new sentry client:", errors.WithStack(err))
		}
		defer sentryClient.Flush(5 * time.Second)

		loggerClient = sentrylogger.New(sentryClient)
	case "rollbar":
		loggerClient = rollbarlogger.New(
			rollbar.New(
				os.Getenv("ROLLBAR_TOKEN"),
				os.Getenv("ENVIRONMENT"),
				"1.0.0",
				os.Getenv("ROLLBAR_SERVERHOST"),
				os.Getenv("ROLLBAR_SERVERROOT"),
			),
		)
	case "zerolog":
		var out io.WriteCloser
		switch os.Getenv("ZEROLOG_OUTPUT") {
		case "STDOUT":
			out = os.Stdout
		case "STDERR":
			fallthrough
		default:
			out = os.Stderr
		}

		zerologLogger := zerolog.New(out)
		loggerClient = zerologlogger.New(zerologLogger)
	default:
		loggerClient = noop.New()
	}

	var underAttackModule *underattack.Dependency = nil
	if *experimentalUnderAttack {
		underAttackDatastoreProvider, ok := os.LookupEnv("UNDER_ATTACK_DATASTORE_PROVIDER")
		if !ok {
			underAttackDatastoreProvider = "memory"
		}

		underAttackDatastoreDSN, ok := os.LookupEnv("UNDER_ATTACK_DATASTORE_DSN")
		if !ok {
			underAttackDatastoreDSN = ""
		}

		var underAttackDatastore underattack.Datastore = nil

		switch strings.ToLower(underAttackDatastoreProvider) {
		case "pgsql":
			fallthrough
		case "postgresql":
			fallthrough
		case "postgres":
			if underAttackDatastoreDSN == "" {
				log.Fatalf("Empty UNDER_ATTACK_DATASTORE_DSN for provider: %s", underAttackDatastoreProvider)
			}

			db, err := sql.Open("postgres", underAttackDatastoreDSN)
			if err != nil {
				log.Fatalf("Creating connection to PostgreSQL: %s", err.Error())
			}

			underAttackDatastore, err = postgres.NewPostgresDatastore(db, loggerClient)
			if err != nil {
				log.Fatalf("Creating NewPostgresDatastore: %s", err.Error())
			}
		case "mysql":
			if underAttackDatastoreDSN == "" {
				log.Fatalf("Empty UNDER_ATTACK_DATASTORE_DSN for provider: %s", underAttackDatastoreProvider)
			}

			db, err := sql.Open("mysql", underAttackDatastoreDSN)
			if err != nil {
				log.Fatalf("Creating connection to PostgreSQL: %s", err.Error())
			}

			underAttackDatastore, err = mysql.NewMySQLDatastore(db, loggerClient)
			if err != nil {
				log.Fatalf("Creating NewPostgresDatastore: %s", err.Error())
			}
		case "memory":
			db, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Hour*24))
			if err != nil {
				log.Fatalf("Creating in memory store: %s", err.Error())
			}

			underAttackDatastore, err = memory.NewInMemoryDatastore(db, loggerClient)
			if err != nil {
				log.Fatalf("Creating NewInMemoryDatastore: %s", err.Error())
			}
		default:
			log.Fatalf("Unknown under attack datastore provider: %s", underAttackDatastoreProvider)
		}

		underAttackModule = &underattack.Dependency{
			Datastore: underAttackDatastore,
		}
	}

	// Setup Telegram Bot
	b, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, ctx tb.Context) {
			if strings.Contains(err.Error(), "Conflict: terminated by other getUpdates request") {
				// This error means the bot is currently being deployed
				return
			}

			loggerClient.HandleError(err)
		},
	})
	if err != nil {
		log.Fatal("during init of bot client:", errors.WithStack(err))
	}
	defer b.Stop()

	// Setup language
	language, ok := os.LookupEnv("LANGUAGE")
	if !ok {
		language = "en"
	}

	// This is for recovering from panic.
	defer func() {
		r := recover()
		if r != nil {
			loggerClient.HandleError(err)

			log.Println(r.(error))
		}
	}()

	deps := cmd.New(cmd.Dependency{
		Memory:      cache,
		Bot:         b,
		Logger:      loggerClient,
		Language:    strings.ToLower(language),
		UnderAttack: underAttackModule,
	})

	// This is basically just for health check.
	b.Handle("/start", func(c tb.Context) error {
		_, err := c.Bot().Send(c.Message().Chat, "ok")
		if err != nil {
			loggerClient.HandleBotError(err, b, c.Message())
		}
		return nil
	})

	// Captcha handlers
	b.Handle(tb.OnUserJoined, deps.OnUserJoinHandler)
	b.Handle(tb.OnText, deps.OnTextHandler)
	b.Handle(tb.OnPhoto, deps.OnNonTextHandler)
	b.Handle(tb.OnAnimation, deps.OnNonTextHandler)
	b.Handle(tb.OnVideo, deps.OnNonTextHandler)
	b.Handle(tb.OnDocument, deps.OnNonTextHandler)
	b.Handle(tb.OnSticker, deps.OnNonTextHandler)
	b.Handle(tb.OnVoice, deps.OnNonTextHandler)
	b.Handle(tb.OnVideoNote, deps.OnNonTextHandler)
	b.Handle(tb.OnUserLeft, deps.OnUserLeftHandler)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill)

	go func() {
		<-signalChan

		log.Println("Shutdown signal received, exiting...")

		if underAttackModule != nil {
			err := underAttackModule.Datastore.Close()
			if err != nil {
				log.Printf("Error during closing datastore connection: %s", err.Error())
			}
		}
	}()

	// Start the bot
	log.Println("Bot started!")
	b.Start()
}
