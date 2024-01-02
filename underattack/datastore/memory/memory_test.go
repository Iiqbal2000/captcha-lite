package memory_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"captcha-lite/logger/noop"
	"captcha-lite/underattack"
	"captcha-lite/underattack/datastore/memory"

	"github.com/allegro/bigcache/v3"
)

var dependency underattack.Datastore

func TestMain(m *testing.M) {
	db, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Hour))
	if err != nil {
		log.Fatalf("Creating bigcache instance: %s", err.Error())
	}

	dependency, err = memory.NewInMemoryDatastore(db, noop.New())
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

func Seed(ctx context.Context, db *bigcache.BigCache) error {
	value, err := json.Marshal(underattack.UnderAttack{
		GroupID:               1,
		IsUnderAttack:         true,
		NotificationMessageID: 1002,
		ExpiresAt:             time.Now().Add(time.Hour),
		UpdatedAt:             time.Now(),
	})
	if err != nil {
		return err
	}

	return db.Set("1", value)
}

func TestNewMySQLDatastore(t *testing.T) {
	t.Run("Nil DB", func(t *testing.T) {
		_, err := memory.NewInMemoryDatastore(nil, nil)
		if err.Error() != "nil db" {
			t.Errorf("expecting an error of 'nil db', instead got %s", err.Error())
		}
	})

	t.Run("Nil logger", func(t *testing.T) {
		_, err := memory.NewInMemoryDatastore(&bigcache.BigCache{}, nil)
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
