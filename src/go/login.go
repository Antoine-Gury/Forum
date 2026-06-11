package handlers

import "net/http"

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
