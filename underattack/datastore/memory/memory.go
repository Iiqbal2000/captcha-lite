package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"captcha-lite/logger"
	"captcha-lite/underattack"

	"github.com/allegro/bigcache/v3"
)

type memoryDatastore struct {
	db     *bigcache.BigCache
	logger logger.Logger
}

func NewInMemoryDatastore(db *bigcache.BigCache, logger logger.Logger) (*memoryDatastore, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}

	if logger == nil {
		return nil, fmt.Errorf("nil logger")
	}

	return &memoryDatastore{db: db, logger: logger}, nil
}

func (m *memoryDatastore) Migrate(ctx context.Context) error {
	// Nothing to migrate
	return nil
}

func (m *memoryDatastore) GetUnderAttackEntry(ctx context.Context, groupID int64) (underattack.UnderAttack, error) {
	value, err := m.db.Get(strconv.FormatInt(groupID, 10))
	if err != nil {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			go func(groupID int64) {
				time.Sleep(time.Second * 5)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
				defer cancel()

				err := m.CreateNewEntry(ctx, groupID)
				if err != nil {
					m.logger.HandleError(err)
				}
			}(groupID)

			return underattack.UnderAttack{}, nil
		}

		return underattack.UnderAttack{}, err
	}

	var entry underattack.UnderAttack
	err = json.Unmarshal(value, &entry)
	if err != nil {
		return underattack.UnderAttack{}, err
	}

	return entry, nil
}

func (m *memoryDatastore) CreateNewEntry(ctx context.Context, groupID int64) error {
	if _, err := m.db.Get(strconv.FormatInt(groupID, 10)); err != nil {
		// Do nothing if already exists
		return nil
	}

	// Set a new one if not exists
	value, err := json.Marshal(underattack.UnderAttack{
		GroupID:               groupID,
		IsUnderAttack:         false,
		NotificationMessageID: 0,
		ExpiresAt:             time.Time{},
		UpdatedAt:             time.Now(),
	})
	if err != nil {
		return err
	}

	return m.db.Set(strconv.FormatInt(groupID, 10), value)
}

func (m *memoryDatastore) SetUnderAttackStatus(ctx context.Context, groupID int64, underAttack bool, expiresAt time.Time, notificationMessageID int64) error {
	// Set a new one if not exists
	value, err := json.Marshal(underattack.UnderAttack{
		GroupID:               groupID,
		IsUnderAttack:         underAttack,
		NotificationMessageID: notificationMessageID,
		ExpiresAt:             expiresAt,
		UpdatedAt:             time.Now(),
	})
	if err != nil {
		return err
	}

	return m.db.Set(strconv.FormatInt(groupID, 10), value)
}

func (m *memoryDatastore) Close() error {
	return m.db.Close()
}
