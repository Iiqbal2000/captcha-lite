package postgres_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"captcha-lite/logger/noop"
	"captcha-lite/underattack"
	"captcha-lite/underattack/datastore/postgres"

	_ "github.com/lib/pq"
)

var dependency underattack.Datastore

func TestMain(m *testing.M) {
	postgresUrl, ok := os.LookupEnv("POSTGRES_URL")
	if !ok {
		postgresUrl = "postgres://captcha:password@localhost:5432/captcha?sslmode=disable"
	}

	db, err := sql.Open("postgres", postgresUrl)
	if err != nil {
		log.Fatalf("opening postgres: %s", err.Error())
	}

	dependency, err = postgres.NewPostgresDatastore(db, noop.New())
	if err != nil {
		log.Fatalf("creating new postgres datastore: %s", err.Error())
	}

	setupCtx, setupCancel := context.WithTimeout(context.Background(), time.Second*30)

	err = dependency.Migrate(setupCtx)
	if err != nil {
		log.Fatalf("migrating tables: %s", err.Error())
	}

	err = Seed(setupCtx, db)
	if err != nil {
		log.Fatalf("seeding data: %s", err.Error())
	}

	exitCode := m.Run()

	setupCancel()

	err = dependency.Close()
	if err != nil {
		log.Printf("closing postgres database: %s", err.Error())
	}

	os.Exit(exitCode)
}

func Seed(ctx context.Context, db *sql.DB) error {
	c, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil {
			log.Print(err)
		}
	}()

	tx, err := c.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO
			under_attack
			(group_id, is_under_attack, expires_at, notification_message_id, updated_at)
			VALUES
			($1, $2, $3, $4, $5)`,
		1,
		true,
		time.Now().Add(time.Hour*1),
		1002,
		time.Now(),
	)
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return e
		}

		return err
	}

	err = tx.Commit()
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return e
		}

		return err
	}

	return nil
}

func TestNewPostgresDatastore(t *testing.T) {
	t.Run("Nil DB", func(t *testing.T) {
		_, err := postgres.NewPostgresDatastore(nil, nil)
		if err.Error() != "nil db" {
			t.Errorf("expecting an error of 'nil db', instead got %s", err.Error())
		}
	})

	t.Run("Nil logger", func(t *testing.T) {
		_, err := postgres.NewPostgresDatastore(&sql.DB{}, nil)
		if err.Error() != "nil logger" {
			t.Errorf("expecting an error of 'nil logger', instead got %s", err.Error())
		}
	})
}

func TestMigrate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	err := dependency.Migrate(ctx)
	if err != nil {
		t.Errorf("migrating database: %s", err.Error())
	}
}

func TestGetUnderAttackEntry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	entry, err := dependency.GetUnderAttackEntry(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if entry.IsUnderAttack == false {
		t.Error("expecting IsUnderAttack to be true, got false")
	}

	if entry.ExpiresAt.Before(time.Now()) {
		t.Errorf("expecting ExpiresAt to be after now, got: %v", entry.ExpiresAt)
	}

	if entry.NotificationMessageID != 1002 {
		t.Errorf("expecting NotificationMessageID to be 1002, got: %v", entry.NotificationMessageID)
	}
}

func TestGetUnderAttackEntry_NotExists(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	_, err := dependency.GetUnderAttackEntry(ctx, 20)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateNewEntry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	err := dependency.CreateNewEntry(ctx, 2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetUnderAttackStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	err := dependency.SetUnderAttackStatus(ctx, 3, true, time.Now().Add(time.Minute*30), 1003)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
