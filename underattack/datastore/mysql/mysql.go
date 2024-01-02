package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"captcha-lite/logger"
	"captcha-lite/underattack"
)

type mysqlDatastore struct {
	db     *sql.DB
	logger logger.Logger
}

func NewMySQLDatastore(db *sql.DB, logger logger.Logger) (*mysqlDatastore, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}

	if logger == nil {
		return nil, fmt.Errorf("nil logger")
	}

	return &mysqlDatastore{db: db, logger: logger}, nil
}

// Migrate will migrates database tables for under attack domain.
func (m *mysqlDatastore) Migrate(ctx context.Context) error {
	c, err := m.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			m.logger.HandleError(err)
		}
	}()

	tx, err := c.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS under_attack (
			group_id BIGINT PRIMARY KEY,
			is_under_attack BOOLEAN NOT NULL,
			expires_at DATETIME NOT NULL,
			notification_message_id BIGINT NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
	)
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return err
		}

		return err
	}
	_, err = tx.ExecContext(
		ctx,
		`CREATE INDEX idx_updated_at ON under_attack (updated_at)`,
	)
	if err != nil && !strings.Contains(err.Error(), "Duplicate key name") {
		if e := tx.Rollback(); e != nil {
			return err
		}

		return err
	}

	err = tx.Commit()
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return err
		}

		return err
	}

	return nil
}

// GetUnderAttackEntry will acquire under attack entry for specified groupID.
func (m *mysqlDatastore) GetUnderAttackEntry(ctx context.Context, groupID int64) (underattack.UnderAttack, error) {
	c, err := m.db.Conn(ctx)
	if err != nil {
		return underattack.UnderAttack{}, err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			m.logger.HandleError(err)
		}
	}()

	tx, err := c.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return underattack.UnderAttack{}, err
	}

	var entry underattack.UnderAttack

	err = tx.QueryRowContext(
		ctx,
		`SELECT
    	group_id,
    	is_under_attack,
    	expires_at,
    	notification_message_id,
    	updated_at
    FROM
        under_attack
    WHERE
        group_id = ?
    ORDER BY
        updated_at DESC`,
		groupID,
	).Scan(
		&entry.GroupID,
		&entry.IsUnderAttack,
		&entry.ExpiresAt,
		&entry.NotificationMessageID,
		&entry.UpdatedAt,
	)
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return underattack.UnderAttack{}, e
		}

		if errors.Is(err, sql.ErrNoRows) {
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

	err = tx.Commit()
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return underattack.UnderAttack{}, e
		}

		return underattack.UnderAttack{}, err
	}

	return entry, nil
}

// CreateNewEntry will create a new entry for given groupID.
// This should only be executed if the group entry does not exists on the database.
// If it already exists, it will do nothing.
func (m *mysqlDatastore) CreateNewEntry(ctx context.Context, groupID int64) error {
	c, err := m.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			m.logger.HandleError(err)
		}
	}()

	tx, err := c.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO
			under_attack
			(group_id, is_under_attack, expires_at, notification_message_id, updated_at)
		VALUES
			(?, ?, ?, ?, ?)
		ON DUPLICATE KEY
		UPDATE
		    group_id = group_id`,
		groupID,
		false,
		time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC),
		0,
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

// SetUnderAttackStatus will update the given groupID entry to the given parameters.
// If the groupID entry does not exists, it will create a new one.
func (m *mysqlDatastore) SetUnderAttackStatus(ctx context.Context, groupID int64, underAttack bool, expiresAt time.Time, notificationMessageID int64) error {
	c, err := m.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			m.logger.HandleError(err)
		}
	}()

	tx, err := c.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO
			under_attack
			(group_id, is_under_attack, expires_at, notification_message_id, updated_at)
		VALUES
			(?, ?, ?, ?, ?)
		ON DUPLICATE KEY
		UPDATE
			is_under_attack = ?,
			expires_at = ?,
			notification_message_id = ?,
			updated_at = ?`,
		groupID,
		underAttack,
		expiresAt,
		notificationMessageID,
		time.Now(),
		underAttack,
		expiresAt,
		notificationMessageID,
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

func (m *mysqlDatastore) Close() error {
	return m.db.Close()
}
