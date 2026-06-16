package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// UploadAvatar gère le POST /upload-avatar
func UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userResult, err := getAuthenticatedUser(w, r)
	if err != nil || userResult.User.Email == "" {
		http.Error(w, "Non connecté", http.StatusUnauthorized)
		return
	}

	// 10 MB max
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Fichier trop volumineux (max 10 MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "Fichier manquant", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Vérifie le type MIME
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
	}
	contentType, ok := allowed[ext]
	if !ok {
		http.Error(w, "Format non supporté (jpg, png, gif, webp)", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Erreur lecture fichier", http.StatusInternalServerError)
		return
	}

	// Upload vers Supabase Storage
	avatarURL, err := uploadToSupabaseStorage(userResult.User.ID, ext, contentType, data)
	if err != nil {
		fmt.Printf("[UploadAvatar] storage error: %v\n", err)
		http.Error(w, "Erreur upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sauvegarde l'URL dans profiles
	if err := SaveAvatarURL(userResult.User.Email, avatarURL); err != nil {
		fmt.Printf("[UploadAvatar] save avatar url error: %v\n", err)
		http.Error(w, "Erreur sauvegarde", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func uploadToSupabaseStorage(userID, ext, contentType string, data []byte) (string, error) {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return "", err
	}

	bucket := os.Getenv("SUPABASE_AVATAR_BUCKET")
	if bucket == "" {
		bucket = "avatars"
	}

	objectPath := "public/" + userID + ext
	uploadURL := supabaseURL + "/storage/v1/object/" + bucket + "/" + objectPath

	// Upsert : écrase si déjà existant
	req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+authKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("storage upload failed %d: %s", resp.StatusCode, string(body))
	}

	// URL publique
	publicURL := supabaseURL + "/storage/v1/object/public/" + bucket + "/" + objectPath
	return publicURL, nil
}
