package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const defaultSupabaseURL = "https://mzeuqpibemltwpwjuqnx.supabase.co"

var httpClient = &http.Client{Timeout: 10 * time.Second}

type supabaseAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	ID           string `json:"id"`
	Email        string `json:"email"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Msg              string `json:"msg"`
}

func getSupabaseURL() string {
	if url := os.Getenv("SUPABASE_URL"); url != "" {
		return url
	}
	return defaultSupabaseURL
}

func getSupabaseAuthKey() (string, error) {
	if key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY"); key != "" {
		return key, nil
	}
	key := os.Getenv("SUPABASE_ANON_KEY")
	if key == "" {
		return "", errors.New("missing SUPABASE_ANON_KEY or SUPABASE_SERVICE_ROLE_KEY environment variable")
	}
	return key, nil
}

func supabaseAuthRequest(path string, payload any, out any) error {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return err
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, supabaseURL+path, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+authKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if out != nil {
		if err := json.Unmarshal(responseBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	if resp.StatusCode >= 300 {
		var errResp supabaseAuthResponse
		if err := json.Unmarshal(responseBody, &errResp); err == nil {
			if errResp.ErrorDescription != "" {
				return errors.New(errResp.ErrorDescription)
			}
			if errResp.Error != "" {
				return errors.New(errResp.Error)
			}
			if errResp.Msg != "" {
				return errors.New(errResp.Msg)
			}
		}
		return fmt.Errorf("supabase auth request failed: %s", resp.Status)
	}

	return nil
}

func persistAuthProfile(result supabaseAuthResponse, username string) error {
	userID := result.User.ID
	if userID == "" {
		userID = result.ID
	}
	email := result.User.Email
	if email == "" {
		email = result.Email
	}
	if userID == "" || email == "" {
		return nil
	}

	if db != nil {
		return SaveProfile(userID, email, username)
	}

	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return err
	}

	payload := map[string]string{"id": userID, "email": email, "username": username}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, supabaseURL+"/rest/v1/profiles", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+authKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("persist profile failed: %s - %s", resp.Status, string(respBody))
	}
	return nil
}

func SignUpUser(email, password, username string) (supabaseAuthResponse, error) {
	if db != nil {
		exists, err := emailAlreadyExists(email)
		if err != nil {
			return supabaseAuthResponse{}, err
		}
		if exists {
			return supabaseAuthResponse{}, errors.New("user already registered")
		}
	}

	payload := map[string]any{
		"email":    email,
		"password": password,
	}
	if username != "" {
		payload["data"] = map[string]string{"username": username}
	}

	var result supabaseAuthResponse
	if err := supabaseAuthRequest("/auth/v1/signup", payload, &result); err != nil {
		return result, err
	}
	if err := persistAuthProfile(result, username); err != nil {
		fmt.Printf("warn: persist profile: %v\n", err)
	}
	return result, nil
}

func SignInUser(email, password string) (supabaseAuthResponse, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	var result supabaseAuthResponse
	if err := supabaseAuthRequest("/auth/v1/token?grant_type=password", payload, &result); err != nil {
		return result, err
	}
	// Pas de persistAuthProfile ici — évite les doublons
	return result, nil
}

func RefreshSession(refreshToken string) (supabaseAuthResponse, error) {
	payload := map[string]string{
		"refresh_token": refreshToken,
	}

	var result supabaseAuthResponse
	if err := supabaseAuthRequest("/auth/v1/token?grant_type=refresh_token", payload, &result); err != nil {
		return result, err
	}
	return result, nil
}

func ExchangeCodeForSession(code string) (supabaseAuthResponse, error) {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return supabaseAuthResponse{}, err
	}

	payload := map[string]string{
		"auth_code": code,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, supabaseURL+"/auth/v1/token?grant_type=pkce", bytes.NewReader(body))
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Println("Supabase PKCE response:", string(respBody))

	var result supabaseAuthResponse
	json.Unmarshal(respBody, &result)

	if resp.StatusCode >= 300 {
		if result.ErrorDescription != "" {
			return result, errors.New(result.ErrorDescription)
		}
		return result, fmt.Errorf("exchange failed: %s", resp.Status)
	}

	return result, nil
}

func SendRecoveryEmail(email string) error {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return err
	}

	payload := map[string]any{
		"email":      email,
		"redirectTo": "http://localhost:8081/auth/callback",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, supabaseURL+"/auth/v1/recover", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+authKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body2, _ := io.ReadAll(resp.Body)
	fmt.Println("Supabase recover response:", string(body2))

	if resp.StatusCode >= 300 {
		return fmt.Errorf("recover failed: %s", resp.Status)
	}

	_ = LogPasswordRecovery(email)
	return nil
}

func GetUserFromToken(token string) (supabaseAuthResponse, error) {
	if token == "" {
		return supabaseAuthResponse{}, errors.New("token manquant")
	}

	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return supabaseAuthResponse{}, err
	}

	req, err := http.NewRequest(http.MethodGet, supabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return supabaseAuthResponse{}, err
	}

	var result supabaseAuthResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return supabaseAuthResponse{}, err
	}

	if result.User.Email == "" {
		result.User.Email = result.Email
		result.User.ID = result.ID
	}

	if resp.StatusCode >= 300 {
		if result.ErrorDescription != "" {
			return result, errors.New(result.ErrorDescription)
		}
		if result.Error != "" {
			return result, errors.New(result.Error)
		}
		return result, fmt.Errorf("supabase user failed: %s", resp.Status)
	}

	return result, nil
}

// InsertDiscussionToSupabase inserts a discussion via Supabase REST API when DB pool is not available.
func InsertDiscussionToSupabase(title, author, content string) (int, error) {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return 0, err
	}
	// Try inserting with English column names first, then without author if required.
	variants := []map[string]string{
		{"title": title, "author": author, "content": content},
		{"title": title, "content": content},
		{"titre": title, "auteur": author, "contenu": content},
		{"titre": title, "contenu": content},
	}

	var respBody []byte
	var lastErr error
	for _, v := range variants {
		body, _ := json.Marshal([]map[string]string{v})

		req, err := http.NewRequest(http.MethodPost, supabaseURL+"/rest/v1/discussions", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("apikey", authKey)
		req.Header.Set("Authorization", "Bearer "+authKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "return=representation")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("insert discussion failed: %s - %s", resp.Status, string(respBody))
			fmt.Printf("[supabase] insert attempt failed (status %s): %s\n", resp.Status, string(respBody))
			continue
		}

		// success
		var rows []struct{ ID int `json:"id"` }
		if err := json.Unmarshal(respBody, &rows); err != nil || len(rows) == 0 {
			// try to parse French/English id key
			var rowsAlt []map[string]any
			if err := json.Unmarshal(respBody, &rowsAlt); err == nil && len(rowsAlt) > 0 {
				if v, ok := rowsAlt[0]["id"].(float64); ok {
					fmt.Printf("[supabase] inserted row id (alt id): %d\n", int(v))
					return int(v), nil
				}
				if v, ok := rowsAlt[0]["identifiant"].(float64); ok {
					fmt.Printf("[supabase] inserted row id (alt identifiant): %d\n", int(v))
					return int(v), nil
				}
			}
			parseErr := fmt.Errorf("unable to parse insert response: %s", string(respBody))
			fmt.Printf("[supabase] %v\n", parseErr)
			return 0, parseErr
		}
		fmt.Printf("[supabase] inserted row id: %d\n", rows[0].ID)
		return rows[0].ID, nil
	}

	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("insert discussion failed: unknown error, no response")
}

// GetDiscussionsFromSupabase fetches discussions via Supabase REST API.
func GetDiscussionsFromSupabase() ([]map[string]any, error) {
	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return nil, err
	}

	// Request all columns and order by id desc. We will map returned keys to expected names.
	req, err := http.NewRequest(http.MethodGet, supabaseURL+"/rest/v1/discussions?select=*&order=id.desc", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+authKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch discussions failed: %s - %s", resp.Status, string(respBody))
	}

	var rows []map[string]any
	if err := json.Unmarshal(respBody, &rows); err != nil {
		return nil, err
	}

	// Normalize keys for compatibility with English/French column names
	for _, r := range rows {
		// id / identifiant
		if _, ok := r["id"]; !ok {
			if v, ok2 := r["identifiant"]; ok2 {
				r["id"] = v
			}
		}
		// title / titre
		if _, ok := r["title"]; !ok {
			if v, ok2 := r["titre"]; ok2 {
				r["title"] = v
			}
		}
		// author / auteur
		if _, ok := r["author"]; !ok {
			if v, ok2 := r["auteur"]; ok2 {
				r["author"] = v
			}
		}
		// content / contenu
		if _, ok := r["content"]; !ok {
			if v, ok2 := r["contenu"]; ok2 {
				r["content"] = v
			}
		}
	}

	return rows, nil
}
