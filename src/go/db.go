package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func InitDB() error {
	dbURL := os.Getenv("SUPABASE_DB_URL")
	if dbURL == "" {
		return errors.New("missing SUPABASE_DB_URL environment variable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("database ping failed: %w", err)
	}

	db = pool
	if err := ensureDiscussionTable(ctx, pool); err != nil {
		pool.Close()
		return err
	}

	if err := ensureAuthTables(ctx, pool); err != nil {
		pool.Close()
		return err
	}

	return nil
}

func ensureDiscussionTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS discussions (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL
		)
	`)
	return err
}

func ensureAuthTables(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'profiles'
				AND column_name = 'email'
				AND data_type != 'text'
			) THEN
				DROP TABLE IF EXISTS profiles;
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("check profiles schema: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			username TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create profiles table: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS password_recovery_requests (
			id SERIAL PRIMARY KEY,
			email TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create password_recovery_requests table: %w", err)
	}

	return nil
}

func SaveProfile(userID, email, username string) error {
	if db == nil {
		return errors.New("database not initialized")
	}
	if userID == "" || email == "" {
		return errors.New("profile data incomplete")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, `
		INSERT INTO profiles (id, email, username, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			username = EXCLUDED.username,
			updated_at = NOW()
	`, userID, email, username)
	return err
}

func LogPasswordRecovery(email string) error {
	if db == nil {
		return errors.New("database not initialized")
	}
	if email == "" {
		return errors.New("email requis")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, `
		INSERT INTO password_recovery_requests (email) VALUES ($1)
	`, email)
	return err
}

func GetDiscussionsFromDB() ([]Discussion, error) {
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.Query(ctx, "SELECT id, title, content FROM discussions ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var discussions []Discussion
	for rows.Next() {
		var d Discussion
		if err := rows.Scan(&d.ID, &d.Title, &d.Content); err != nil {
			return nil, err
		}
		discussions = append(discussions, d)
	}

	return discussions, rows.Err()
}

func InsertDiscussion(title, content string) error {
	if db == nil {
		return errors.New("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, "INSERT INTO discussions (title, content) VALUES ($1, $2)", title, content)
	return err
}

func GetDiscussionByID(id int) (Discussion, error) {
	if db == nil {
		return Discussion{}, errors.New("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var d Discussion
	err := db.QueryRow(ctx, "SELECT id, title, content FROM discussions WHERE id = $1", id).Scan(&d.ID, &d.Title, &d.Content)
	return d, err
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}
