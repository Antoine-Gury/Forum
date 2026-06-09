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

var discussions = []Discussion{
	{ID: 0, Title: "Raid ICC stratégie", Content: "Comment battre le Roi Liche ?"},
}

func render(w http.ResponseWriter, page string, data interface{}) {
	tpl := template.Must(template.ParseFiles("templates/" + page))
	tpl.Execute(w, data)
}

func Home(w http.ResponseWriter, r *http.Request) {
    render(w, "index.html", discussions)
}

func Profil(w http.ResponseWriter, r *http.Request) {
    render(w, "profil.html", nil)
}

func Create(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		title := r.FormValue("title")
		content := r.FormValue("content")

		id := len(discussions)

		discussions = append(discussions, Discussion{
			ID:      id,
			Title:   title,
			Content: content,
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	render(w, "create.html", nil)
}

func DiscussionPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	for _, d := range discussions {
		if d.ID == id {
			render(w, "discussion.html", d)
			return
		}
	}

	http.NotFound(w, r)
}

