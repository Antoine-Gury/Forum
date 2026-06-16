package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
		var rows []struct {
			ID int `json:"id"`
		}
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
