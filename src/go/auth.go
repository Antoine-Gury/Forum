package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultSupabaseURL = "https://mzeuqpibemltwpwjuqnx.supabase.co"

var httpClient = &http.Client{Timeout: 10 * time.Second}

var authAttemptLimiter = struct {
	mu       sync.Mutex
	attempts map[string]time.Time
}{
	attempts: make(map[string]time.Time),
}

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

type authPageData struct {
	Message string
	Error   string
}

func getSupabaseURL() string {
	if url := os.Getenv("SUPABASE_URL"); url != "" {
		return url
	}
	return defaultSupabaseURL
}

func getSupabaseAnonKey() (string, error) {
	key := os.Getenv("SUPABASE_ANON_KEY")
	if key == "" {
		return "", errors.New("missing SUPABASE_ANON_KEY environment variable")
	}
	return key, nil
}

func supabaseAuthRequest(path string, payload any, out any) error {
	supabaseURL := getSupabaseURL()
	anonKey, err := getSupabaseAnonKey()
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
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)
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
	return SaveProfile(userID, email, username)
}

func SignUpUser(email, password, username string) (supabaseAuthResponse, error) {
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
		return result, err
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
	if err := persistAuthProfile(result, ""); err != nil {
		return result, err
	}
	return result, nil
}

func SendRecoveryEmail(email string) error {
	payload := map[string]string{
		"email": email,
	}

	var result supabaseAuthResponse
	if err := supabaseAuthRequest("/auth/v1/recover", payload, &result); err != nil {
		return err
	}
	if err := LogPasswordRecovery(email); err != nil {
		return err
	}
	return nil
}

func GetUserFromToken(token string) (supabaseAuthResponse, error) {
	if token == "" {
		return supabaseAuthResponse{}, errors.New("token manquant")
	}

	supabaseURL := getSupabaseURL()
	anonKey, err := getSupabaseAnonKey()
	if err != nil {
		return supabaseAuthResponse{}, err
	}

	req, err := http.NewRequest(http.MethodGet, supabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	req.Header.Set("apikey", anonKey)
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

func friendlyAuthError(err error) string {
	if err == nil {
		return ""
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "email rate limit exceeded"), strings.Contains(msg, "rate limit exceeded"):
		return "Trop de tentatives. Utilise une autre adresse email ou attends quelques minutes avant de réessayer."
	case strings.Contains(msg, "invalid login credentials"), strings.Contains(msg, "invalid credentials"):
		return "Identifiants invalides. Vérifie ton email et ton mot de passe."
	case strings.Contains(msg, "user already registered"), strings.Contains(msg, "user already exists"):
		return "Cet email est déjà utilisé. Connecte-toi ou utilise un autre email."
	default:
		return err.Error()
	}
}

func checkAuthRateLimit(r *http.Request, email string) (bool, string) {
	key := r.RemoteAddr + "|" + strings.ToLower(strings.TrimSpace(email))
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		key = strings.TrimSpace(strings.Split(xff, ",")[0]) + "|" + strings.ToLower(strings.TrimSpace(email))
	}

	authAttemptLimiter.mu.Lock()
	defer authAttemptLimiter.mu.Unlock()

	now := time.Now()
	last, exists := authAttemptLimiter.attempts[key]
	if exists && now.Sub(last) < 30*time.Second {
		return false, "Trop de tentatives trop rapides. Attends 30 secondes avant de réessayer."
	}

	authAttemptLimiter.attempts[key] = now
	return true, ""
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		render(w, "Login.html", authPageData{Error: "Email et mot de passe requis."})
		return
	}

	if allowed, msg := checkAuthRateLimit(r, email); !allowed {
		render(w, "Login.html", authPageData{Error: msg})
		return
	}

	result, err := SignInUser(email, password)
	if err != nil {
		render(w, "Login.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	if result.AccessToken == "" {
		render(w, "Login.html", authPageData{Error: "Impossible de se connecter. Vérifie tes identifiants."})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sb-access-token",
		Value:    result.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "sb-refresh-token",
		Value:    result.RefreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/profil", http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	expire := time.Unix(0, 0)

	http.SetCookie(w, &http.Cookie{
		Name:     "sb-access-token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expire,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-refresh-token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expire,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	username := r.FormValue("username")
	if email == "" || password == "" {
		render(w, "register.html", authPageData{Error: "Email et mot de passe requis."})
		return
	}

	if allowed, msg := checkAuthRateLimit(r, email); !allowed {
		render(w, "register.html", authPageData{Error: msg})
		return
	}

	_, err := SignUpUser(email, password, username)
	if err != nil {
		render(w, "register.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	render(w, "register.html", authPageData{Message: "Inscription enregistrée. Vérifie tes mails pour confirmer ton compte."})
}

func ForgotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/forgot", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		render(w, "Forget_passwd.html", authPageData{Error: "Email requis."})
		return
	}

	if allowed, msg := checkAuthRateLimit(r, email); !allowed {
		render(w, "Forget_passwd.html", authPageData{Error: msg})
		return
	}

	if err := SendRecoveryEmail(email); err != nil {
		render(w, "Forget_passwd.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	render(w, "Forget_passwd.html", authPageData{Message: "Un email de récupération a été envoyé."})
}
