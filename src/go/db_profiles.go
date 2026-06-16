package handlers

import (
	"context"
	"errors"
	"time"
)

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
