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
	"time"
)

const defaultSupabaseURL = "https://mzeuqpibemltwpwjuqnx.supabase.co"

const cookieMaxAge = 7 * 24 * 60 * 60

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

type authPageData struct {
	Message string
	Error   string
	Token   string
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
	_ = persistAuthProfile(result, username)
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
	_ = persistAuthProfile(result, "")
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
	case strings.Contains(msg, "email not confirmed"):
		return "Ton email n'est pas encore confirmé. Vérifie ta boîte mail."
	default:
		return err.Error()
	}
}

func setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-access-token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   cookieMaxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "sb-refresh-token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   cookieMaxAge,
	})
}

func getAuthenticatedUser(w http.ResponseWriter, r *http.Request) (supabaseAuthResponse, error) {
	accessToken := ""
	if c, err := r.Cookie("sb-access-token"); err == nil {
		accessToken = c.Value
	}

	refreshToken := ""
	if c, err := r.Cookie("sb-refresh-token"); err == nil {
		refreshToken = c.Value
	}

	if accessToken == "" && refreshToken == "" {
		return supabaseAuthResponse{}, errors.New("non connecté")
	}

	if accessToken != "" {
		user, err := GetUserFromToken(accessToken)
		if err == nil && user.User.Email != "" {
			return user, nil
		}
	}

	if refreshToken == "" {
		return supabaseAuthResponse{}, errors.New("session expirée")
	}

	refreshed, err := RefreshSession(refreshToken)
	if err != nil || refreshed.AccessToken == "" {
		return supabaseAuthResponse{}, errors.New("session expirée, reconnecte-toi")
	}

	setAuthCookies(w, refreshed.AccessToken, refreshed.RefreshToken)

	user, err := GetUserFromToken(refreshed.AccessToken)
	if err != nil {
		return supabaseAuthResponse{}, err
	}
	return user, nil
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

	result, err := SignInUser(email, password)
	if err != nil {
		render(w, "Login.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	if result.AccessToken == "" {
		render(w, "Login.html", authPageData{Error: "Impossible de se connecter. Vérifie tes identifiants."})
		return
	}

	setAuthCookies(w, result.AccessToken, result.RefreshToken)
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

	_, err := SignUpUser(email, password, username)
	if err != nil {
		render(w, "register.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	render(w, "register.html", authPageData{Message: "Inscription réussie ! Vérifie ta boîte mail et clique sur le lien de confirmation pour accéder au site."})
}

func AuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	tokenType := r.URL.Query().Get("type")

	if code != "" && tokenType == "recovery" {
		// Échange du code PKCE
		result, err := ExchangeCodeForSession(code)
		if err != nil || result.AccessToken == "" {
			render(w, "Login.html", authPageData{Error: "Lien invalide ou expiré."})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "sb-reset-token",
			Value:    result.AccessToken,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   600,
		})
		http.Redirect(w, r, "/reset", http.StatusSeeOther)
		return
	}

	if code != "" {
		result, err := ExchangeCodeForSession(code)
		if err != nil || result.AccessToken == "" {
			render(w, "Login.html", authPageData{Error: "Lien de confirmation invalide ou expiré."})
			return
		}
		_ = persistAuthProfile(result, "")
		setAuthCookies(w, result.AccessToken, result.RefreshToken)
		http.Redirect(w, r, "/profil", http.StatusSeeOther)
		return
	}

	// Pas de code = hash fragment (confirmation email) → page intermédiaire JS
	render(w, "auth_callback.html", nil)
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

	if err := SendRecoveryEmail(email); err != nil {
		render(w, "Forget_passwd.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	render(w, "Forget_passwd.html", authPageData{Message: "Un email de récupération a été envoyé."})
}

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	render(w, "forgot.html", authPageData{})
}

func ResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/reset", http.StatusSeeOther)
		return
	}

	cookie, err := r.Cookie("sb-reset-token")
	if err != nil || cookie.Value == "" {
		render(w, "forgot.html", authPageData{Error: "Session expirée. Refais une demande de réinitialisation."})
		return
	}

	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	if password != confirm {
		render(w, "forgot.html", authPageData{Error: "Les mots de passe ne correspondent pas."})
		return
	}
	if len(password) < 8 {
		render(w, "forgot.html", authPageData{Error: "Minimum 8 caractères."})
		return
	}

	if err := UpdatePassword(cookie.Value, password); err != nil {
		render(w, "forgot.html", authPageData{Error: friendlyAuthError(err)})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "sb-reset-token", Value: "", Path: "/", MaxAge: -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func UpdatePassword(accessToken, newPassword string) error {
	payload := map[string]string{"password": newPassword}

	supabaseURL := getSupabaseURL()
	authKey, err := getSupabaseAuthKey()
	if err != nil {
		return err
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPut, supabaseURL+"/auth/v1/user", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", authKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("échec de la mise à jour du mot de passe")
	}
	return nil
}
