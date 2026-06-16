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

func InsertReply(discussionID int, author, content, avatarURL string) (Reply, error) {
	if db == nil {
		return Reply{}, errors.New("database not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	var createdAt time.Time
	err := db.QueryRow(ctx,
		`INSERT INTO discussion_replies (discussion_id, author, content, avatar_url) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		discussionID, author, content, avatarURL,
	).Scan(&id, &createdAt)
	if err != nil {
		return Reply{}, err
	}

	return Reply{ID: id, DiscussionID: discussionID, Author: author, Content: content, AvatarURL: avatarURL, CreatedAt: createdAt.Format(time.RFC3339)}, nil
}

func GetRepliesByDiscussionID(discussionID int) ([]Reply, error) {
	if db == nil {
		return nil, errors.New("database not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.Query(ctx, `
		SELECT id, discussion_id, author, content, COALESCE(avatar_url, ''), created_at
		FROM discussion_replies
		WHERE discussion_id = $1
		ORDER BY created_at ASC
	`, discussionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var replies []Reply
	for rows.Next() {
		var r Reply
		var createdAt time.Time
		if err := rows.Scan(&r.ID, &r.DiscussionID, &r.Author, &r.Content, &r.AvatarURL, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt = createdAt.Format(time.RFC3339)
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

func UpsertVote(discussionID int, userID string, value int) (int, error) {
	if db == nil {
		return 0, errors.New("database not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if value == 0 {
		if _, err := db.Exec(ctx,
			`DELETE FROM discussion_votes WHERE discussion_id=$1 AND user_id=$2`,
			discussionID, userID); err != nil {
			return 0, err
		}
	} else {
		if _, err := db.Exec(ctx, `
			INSERT INTO discussion_votes (discussion_id, user_id, value)
			VALUES ($1, $2, $3)
			ON CONFLICT (discussion_id, user_id) DO UPDATE SET value = EXCLUDED.value
		`, discussionID, userID, value); err != nil {
			return 0, err
		}
	}

	var score int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(value),0) FROM discussion_votes WHERE discussion_id=$1`,
		discussionID,
	).Scan(&score)
	return score, err
}

func GetUserVote(discussionID int, userID string) int {
	if db == nil || userID == "" {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var v int
	_ = db.QueryRow(ctx,
		`SELECT value FROM discussion_votes WHERE discussion_id=$1 AND user_id=$2`,
		discussionID, userID,
	).Scan(&v)
	return v
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

func SaveAvatarURL(email, avatarURL string) error {
	if db == nil {
		return errors.New("database not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, `
		UPDATE profiles SET avatar_url = $1, updated_at = NOW() WHERE email = $2
	`, avatarURL, email)
	return err
}

func GetAvatarURLByEmail(email string) string {
	if db == nil || email == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var url string
	_ = db.QueryRow(ctx,
		"SELECT COALESCE(avatar_url, '') FROM profiles WHERE email = $1", email,
	).Scan(&url)
	return url
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
	if db == nil || userID == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var username string
	_ = db.QueryRow(ctx,
		"SELECT COALESCE(username, '') FROM profiles WHERE id = $1", userID,
	).Scan(&username)
	return username
}

func GetUsernameByEmail(email string) string {
	if db == nil || email == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var username string
	_ = db.QueryRow(ctx,
		"SELECT COALESCE(username, '') FROM profiles WHERE email = $1", email,
	).Scan(&username)
	return username
}

// GetDiscussionsFromDB récupère les discussions en joignant l'avatar du profil
// par username, ainsi que le score de vote et le vote de l'utilisateur courant.
func GetDiscussionsFromDB(userID string) ([]Discussion, error) {
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

	rows, err := db.Query(ctx, `
		SELECT
			d.id,
			d.title,
			d.author,
			d.content,
			COALESCE(NULLIF(p.avatar_url, ''), d.avatar_url, '') AS avatar_url,
			COALESCE(v.score, 0) AS score,
			COALESCE(uv.value, 0) AS user_vote,
			d.created_at
		FROM discussions d
		LEFT JOIN profiles p ON p.username = d.author
		LEFT JOIN (
			SELECT discussion_id, SUM(value) AS score
			FROM discussion_votes
			GROUP BY discussion_id
		) v ON v.discussion_id = d.id
		LEFT JOIN discussion_votes uv ON uv.discussion_id = d.id AND uv.user_id = $1
		ORDER BY COALESCE(v.score, 0) DESC, d.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var discussions []Discussion
	for rows.Next() {
		var d Discussion
		var createdAt time.Time
		if err := rows.Scan(&d.ID, &d.Title, &d.Author, &d.Content, &d.AvatarURL, &d.Score, &d.UserVote, &createdAt); err != nil {
			return nil, err
		}
		d.CreatedAt = createdAt.Format(time.RFC3339)
		discussions = append(discussions, d)
	}

	return discussions, rows.Err()
}

func InsertDiscussion(title, author, content, avatarURL string) (Discussion, error) {
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
	err := db.QueryRow(ctx,
		"INSERT INTO discussions (title, author, content, avatar_url) VALUES ($1, $2, $3, $4) RETURNING id",
		title, author, content, avatarURL,
	).Scan(&id)
	if err != nil {
		return Discussion{}, err
	}

	return Discussion{ID: id, Title: title, Author: author, Content: content, AvatarURL: avatarURL, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func GetDiscussionByID(id int, userID string) (Discussion, error) {
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

	// previously built `query` variable removed because a full query is used below

	var d Discussion
	var createdAt time.Time
	err := db.QueryRow(ctx, `
		SELECT
			d.id,
			d.title,
			d.author,
			d.content,
			COALESCE(NULLIF(p.avatar_url, ''), d.avatar_url, '') AS avatar_url,
			COALESCE(v.score, 0) AS score,
			COALESCE(uv.value, 0) AS user_vote,
			d.created_at
		FROM discussions d
		LEFT JOIN profiles p ON p.username = d.author
		LEFT JOIN (
			SELECT discussion_id, SUM(value) AS score
			FROM discussion_votes
			GROUP BY discussion_id
		) v ON v.discussion_id = d.id
		LEFT JOIN discussion_votes uv ON uv.discussion_id = d.id AND uv.user_id = $1
		WHERE d.id = $2
	`, userID, id).Scan(&d.ID, &d.Title, &d.Author, &d.Content, &d.AvatarURL, &d.Score, &d.UserVote, &createdAt)
	d.CreatedAt = createdAt.Format(time.RFC3339)
	if err != nil {
		return d, err
	}

	// load replies for this discussion
	if replies, rerr := GetRepliesByDiscussionID(id); rerr == nil {
		d.Replies = replies
	}

	return d, nil
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}
