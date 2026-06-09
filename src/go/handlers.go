package handlers

import (
	"html/template"
	"net/http"
	"strconv"
)

type Discussion struct {
	ID      int
	Title   string
	Content string
}

type ProfilePageData struct {
	Email string
	ID    string
	Error string
}

type CreatePageData struct {
	Error string
}

var defaultDiscussions = []Discussion{
	{ID: 0, Title: "Raid ICC stratégie", Content: "Comment battre le Roi Liche ?"},
}

func render(w http.ResponseWriter, page string, data interface{}) {
	tpl := template.Must(template.ParseFiles("templates/" + page))
	tpl.Execute(w, data)
}

func getDiscussions() []Discussion {
	discussions, err := GetDiscussionsFromDB()
	if err != nil || len(discussions) == 0 {
		return defaultDiscussions
	}
	return discussions
}

func Home(w http.ResponseWriter, r *http.Request) {
	render(w, "index.html", getDiscussions())
}

func getAccessTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie("sb-access-token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func Profil(w http.ResponseWriter, r *http.Request) {
	token := getAccessTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userResult, err := GetUserFromToken(token)
	if err != nil || userResult.User.Email == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	render(w, "profil.html", ProfilePageData{
		Email: userResult.User.Email,
		ID:    userResult.User.ID,
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	render(w, "Login.html", nil)
}

func Register(w http.ResponseWriter, r *http.Request) {
	render(w, "register.html", nil)
}

func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	render(w, "Forget_passwd.html", nil)
}

func Create(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		title := r.FormValue("title")
		content := r.FormValue("content")

		if err := InsertDiscussion(title, content); err != nil {
			render(w, "create.html", CreatePageData{Error: err.Error()})
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	render(w, "create.html", nil)
}

func DiscussionPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	if discussion, err := GetDiscussionByID(id); err == nil {
		render(w, "discussion.html", discussion)
		return
	}

	for _, d := range getDiscussions() {
		if d.ID == id {
			render(w, "discussion.html", d)
			return
		}
	}

	http.NotFound(w, r)
}
