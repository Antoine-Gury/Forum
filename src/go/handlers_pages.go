package handlers

import (
	"html/template"
	"net/http"
)

var defaultDiscussions = []Discussion{
	{ID: 0, Title: "Raid ICC stratégie", Author: "Ezekiel", Content: "Comment battre le Roi Liche ?", AvatarURL: ""},
}

func render(w http.ResponseWriter, page string, data interface{}) {
	tpl := template.Must(template.ParseFiles("templates/" + page))
	tpl.Execute(w, data)
}

func currentUserID(w http.ResponseWriter, r *http.Request) string {
	userResult, err := getAuthenticatedUser(w, r)
	if err != nil {
		return ""
	}
	return userResult.User.ID
}

func getDiscussions(userID string) []Discussion {
	discussions, err := GetDiscussionsFromDB(userID)
	if err != nil || len(discussions) == 0 {
		return defaultDiscussions
	}
	return discussions
}

func Home(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Query().Get("message")
	userID := currentUserID(w, r)
	render(w, "index.html", HomePageData{
		Discussions: getDiscussions(userID),
		Message:     message,
	})
}

func Profil(w http.ResponseWriter, r *http.Request) {
	userResult, err := getAuthenticatedUser(w, r)
	if err != nil || userResult.User.Email == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	email := userResult.User.Email
	username := GetUsernameByEmail(email)
	if username == "" {
		username = GetUsernameByID(userResult.User.ID)
	}
	avatarURL := GetAvatarURLByEmail(email)

	render(w, "profil.html", ProfilePageData{
		Email:     email,
		ID:        userResult.User.ID,
		Username:  username,
		AvatarURL: avatarURL,
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	render(w, "Login.html", nil)
}

func Register(w http.ResponseWriter, r *http.Request) {
	render(w, "register.html", nil)
}
