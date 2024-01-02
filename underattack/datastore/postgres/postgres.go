package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"captcha-lite/logger"
	"captcha-lite/underattack"
)

type postgresDatastore struct {
	db     *sql.DB
	logger logger.Logger
}

func NewPostgresDatastore(db *sql.DB, logger logger.Logger) (*postgresDatastore, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}

	if logger == nil {
		return nil, fmt.Errorf("nil logger")
	}

	return &postgresDatastore{db: db, logger: logger}, nil
}

// Migrate will migrates database tables for under attack domain.
func (p *postgresDatastore) Migrate(ctx context.Context) error {
	c, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			p.logger.HandleError(err)
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
			expires_at TIMESTAMP NOT NULL,
			notification_message_id BIGINT NOT NULL,
			updated_at TIMESTAMP NOT NULL
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
		`CREATE INDEX IF NOT EXISTS idx_updated_at ON under_attack (updated_at)`,
	)
	if err != nil {
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
func (p *postgresDatastore) GetUnderAttackEntry(ctx context.Context, groupID int64) (underattack.UnderAttack, error) {
	c, err := p.db.Conn(ctx)
	if err != nil {
		return underattack.UnderAttack{}, err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			p.logger.HandleError(err)
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
        group_id = $1
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

				err := p.CreateNewEntry(ctx, groupID)
				if err != nil {
					p.logger.HandleError(err)
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
func (p *postgresDatastore) CreateNewEntry(ctx context.Context, groupID int64) error {
	c, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			p.logger.HandleError(err)
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
			($1, $2, $3, $4, $5)
		ON CONFLICT (group_id)
		DO NOTHING`,
		groupID,
		false,
		time.Time{},
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
func (p *postgresDatastore) SetUnderAttackStatus(ctx context.Context, groupID int64, underAttack bool, expiresAt time.Time, notificationMessageID int64) error {
	c, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil && !errors.Is(err, sql.ErrConnDone) {
			p.logger.HandleError(err)
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
			($1, $2, $3, $4, $5)
		ON CONFLICT (group_id)
		DO UPDATE
		SET
			is_under_attack = $2,
			expires_at = $3,
			notification_message_id = $4,
			updated_at = $5`,
		groupID,
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

func (p *postgresDatastore) Close() error {
	return p.db.Close()
}
