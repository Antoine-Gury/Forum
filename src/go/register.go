package handlers

import "net/http"

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
