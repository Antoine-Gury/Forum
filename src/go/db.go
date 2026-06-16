package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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

	return nil
}

func ensureDiscussionTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS discussions (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			author TEXT NOT NULL DEFAULT 'Invité',
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
			username = COALESCE(NULLIF(EXCLUDED.username, ''), profiles.username),
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

func GetUsernameByID(userID string) string {
	if db == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var username string
	err := db.QueryRow(ctx,
		"SELECT COALESCE(username, '') FROM profiles WHERE id = $1", userID,
	).Scan(&username)
	if err != nil {
		return ""
	}
	return username
}

func GetUsernameByEmail(email string) string {
	if db == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var username string
	err := db.QueryRow(ctx,
		"SELECT COALESCE(username, '') FROM profiles WHERE email = $1", email,
	).Scan(&username)
	if err != nil {
		return ""
	}
	return username
}
func GetDiscussionsFromDB() ([]Discussion, error) {

	if db == nil {
		// fallback to Supabase REST API
		rows, err := GetDiscussionsFromSupabase()
		if err != nil {
			return nil, errors.New("database not initialized")
		}
		var discussions []Discussion
		for _, r := range rows {
			var d Discussion
			if v, ok := r["id"].(float64); ok {
				d.ID = int(v)
			} else if v, ok := r["identifiant"].(float64); ok {
				d.ID = int(v)
			}
			if v, ok := r["title"].(string); ok {
				d.Title = v
			} else if v, ok := r["titre"].(string); ok {
				d.Title = v
			}
			if v, ok := r["author"].(string); ok {
				d.Author = v
			} else if v, ok := r["auteur"].(string); ok {
				d.Author = v
			}
			if v, ok := r["content"].(string); ok {
				d.Content = v
			} else if v, ok := r["contenu"].(string); ok {
				d.Content = v
			}
			discussions = append(discussions, d)
		}
		return discussions, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cols := []string{discussionSchema.idCol}
	if discussionSchema.hasTitle {
		cols = append(cols, discussionSchema.titleCol)
	}
	if discussionSchema.hasAuthor {
		cols = append(cols, discussionSchema.authorCol)
	}
	if discussionSchema.hasContent {
		cols = append(cols, discussionSchema.contentCol)
	}

	query := fmt.Sprintf("SELECT %s FROM discussions ORDER BY %s DESC", strings.Join(cols, ", "), discussionSchema.idCol)
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var discussions []Discussion
	for rows.Next() {
		var d Discussion
		scanArgs := []interface{}{&d.ID}
		if discussionSchema.hasTitle {
			scanArgs = append(scanArgs, &d.Title)
		}
		if discussionSchema.hasAuthor {
			scanArgs = append(scanArgs, &d.Author)
		}
		if discussionSchema.hasContent {
			scanArgs = append(scanArgs, &d.Content)
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		discussions = append(discussions, d)
	}

	return discussions, rows.Err()
}

func InsertDiscussion(title, author, content string) (Discussion, error) {
	if db == nil {
		// Fallback to Supabase REST API when DB pool isn't initialized
		if id, err := InsertDiscussionToSupabase(title, author, content); err == nil {
			return Discussion{ID: id, Title: title, Author: author, Content: content}, nil
		} else {
			return Discussion{}, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	cols := []string{discussionSchema.titleCol, discussionSchema.contentCol}
	args := []interface{}{title, content}
	if discussionSchema.hasAuthor {
		cols = append(cols, discussionSchema.authorCol)
		args = append(args, author)
	}

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("INSERT INTO discussions (%s) VALUES (%s) RETURNING %s", strings.Join(cols, ", "), strings.Join(placeholders, ", "), discussionSchema.idCol)
	err := db.QueryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		return Discussion{}, err
	}

	return Discussion{ID: id, Title: title, Author: author, Content: content}, nil
}

func GetDiscussionByID(id int) (Discussion, error) {
	if db == nil {
		return Discussion{}, errors.New("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cols := []string{discussionSchema.idCol}
	if discussionSchema.hasTitle {
		cols = append(cols, discussionSchema.titleCol)
	}
	if discussionSchema.hasAuthor {
		cols = append(cols, discussionSchema.authorCol)
	}
	if discussionSchema.hasContent {
		cols = append(cols, discussionSchema.contentCol)
	}

	query := fmt.Sprintf("SELECT %s FROM discussions WHERE %s = $1", strings.Join(cols, ", "), discussionSchema.idCol)

	var d Discussion
	scanArgs := []interface{}{&d.ID}
	if discussionSchema.hasTitle {
		scanArgs = append(scanArgs, &d.Title)
	}
	if discussionSchema.hasAuthor {
		scanArgs = append(scanArgs, &d.Author)
	}
	if discussionSchema.hasContent {
		scanArgs = append(scanArgs, &d.Content)
	}

	err := db.QueryRow(ctx, query, id).Scan(scanArgs...)
	return d, err
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}
