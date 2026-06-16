package handlers

import (
	"context"
	"errors"
	"time"
)

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
