package handlers

import (
	"errors"
	"net/http"
	"strings"
)

const cookieMaxAge = 7 * 24 * 60 * 60

type authPageData struct {
	Message string
	Error   string
	Token   string
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

func AuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")

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
