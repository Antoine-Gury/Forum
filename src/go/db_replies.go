package handlers

import (
	"context"
	"errors"
	"time"
)

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
