package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spacebxr/strelp/internal/models"
)

type Database struct {
	pool *pgxpool.Pool
}

func NewDatabase(connString string) (*Database, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	// Ping the database
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("could not ping database: %v", err)
	}

	return &Database{pool: pool}, nil
}

func (db *Database) SetPresence(ctx context.Context, userID string, presence *models.Presence) error {
	data, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO presences (user_id, data, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET data = EXCLUDED.data, updated_at = NOW();
	`

	_, err = db.pool.Exec(ctx, query, userID, data)
	return err
}

func (db *Database) GetPresence(ctx context.Context, userID string) (*models.Presence, error) {
	var data []byte
	query := "SELECT data FROM presences WHERE user_id = $1"

	err := db.pool.QueryRow(ctx, query, userID).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("presence not found")
		}
		return nil, err
	}

	var presence models.Presence
	if err := json.Unmarshal(data, &presence); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (db *Database) DeletePresence(ctx context.Context, userID string) error {
	query := "DELETE FROM presences WHERE user_id = $1"
	_, err := db.pool.Exec(ctx, query, userID)
	return err
}

func (db *Database) AcquireConn(ctx context.Context) (*pgxpool.Conn, error) {
	return db.pool.Acquire(ctx)
}

func (db *Database) Close() {
	db.pool.Close()
}
