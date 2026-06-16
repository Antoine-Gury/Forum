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

var discussionSchema = struct {
	idCol      string
	titleCol   string
	authorCol  string
	contentCol string
	hasAuthor  bool
	hasTitle   bool
	hasContent bool
}{
	idCol:      "id",
	titleCol:   "title",
	authorCol:  "author",
	contentCol: "content",
	hasAuthor:  true,
	hasTitle:   true,
	hasContent: true,
}

func loadDiscussionSchema(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT LOWER(column_name)
		FROM information_schema.columns
		WHERE table_name = 'discussions'
		AND table_schema = 'public'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasID := false
	hasTitle := false
	hasAuthor := false
	hasContent := false

	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return err
		}
		switch column {
		case "id", "identifiant":
			discussionSchema.idCol = column
			hasID = true
		case "title", "titre":
			discussionSchema.titleCol = column
			hasTitle = true
		case "author", "auteur":
			discussionSchema.authorCol = column
			hasAuthor = true
		case "content", "contenu":
			discussionSchema.contentCol = column
			hasContent = true
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if !hasID {
		return errors.New("discussions table missing id/identifiant column")
	}
	if !hasTitle {
		return errors.New("discussions table missing title/titre column")
	}
	if !hasContent {
		return errors.New("discussions table missing content/contenu column")
	}

	discussionSchema.hasTitle = hasTitle
	discussionSchema.hasAuthor = hasAuthor
	discussionSchema.hasContent = hasContent
	fmt.Printf("[db] discussion schema: id=%s title=%s author=%s content=%s\n", discussionSchema.idCol, discussionSchema.titleCol, discussionSchema.authorCol, discussionSchema.contentCol)
	return nil
}

func emailAlreadyExists(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := db.QueryRow(ctx,
		"SELECT COUNT(*) FROM profiles WHERE email = $1", email,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

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

	if err := loadDiscussionSchema(ctx, pool); err != nil {
		pool.Close()
		return err
	}

	if err := ensureAuthTables(ctx, pool); err != nil {
		pool.Close()
		return err
	}

	if err := ensureVotesTable(ctx, pool); err != nil {
		pool.Close()
		return err
	}

	if err := ensureRepliesTable(ctx, pool); err != nil {
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
			author TEXT NOT NULL DEFAULT 'Invité',
			content TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `ALTER TABLE discussions ADD COLUMN IF NOT EXISTS author TEXT NOT NULL DEFAULT 'Invité'`)
	_, _ = pool.Exec(ctx, `ALTER TABLE discussions ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT ''`)
	_, _ = pool.Exec(ctx, `ALTER TABLE discussions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`)
	return nil
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
			avatar_url TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create profiles table: %w", err)
	}

	_, _ = pool.Exec(ctx, `ALTER TABLE profiles ADD COLUMN IF NOT EXISTS avatar_url TEXT`)

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

func ensureVotesTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS discussion_votes (
			discussion_id INT NOT NULL REFERENCES discussions(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			value SMALLINT NOT NULL CHECK (value IN (-1,1)),
			PRIMARY KEY (discussion_id, user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("create discussion_votes table: %w", err)
	}
	return nil
}

func ensureRepliesTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS discussion_replies (
			id SERIAL PRIMARY KEY,
			discussion_id INT NOT NULL REFERENCES discussions(id) ON DELETE CASCADE,
			author TEXT NOT NULL DEFAULT 'Invité',
			content TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create discussion_replies table: %w", err)
	}
	return nil
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

func CloseDB() {
	if db != nil {
		db.Close()
	}
}

// Autres fichiers contiennent:
// - db_discussions.go: Opérations sur discussions
// - db_profiles.go: Opérations sur profils
// - db_replies.go: Opérations sur réponses
// - db_votes.go: Opérations sur votes
