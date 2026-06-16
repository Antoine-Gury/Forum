package handlers

import (
	"context"
	"errors"
	"time"
)

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
