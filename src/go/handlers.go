package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type Discussion struct {
	ID      int
	Title   string
	Author  string
	Content string
}

type HomePageData struct {
	Discussions []Discussion
	Message     string
}

type ProfilePageData struct {
	Email    string
	ID       string
	Username string
	Error    string
}

type CreatePageData struct {
	Error string
}

var defaultDiscussions = []Discussion{
	{ID: 0, Title: "Raid ICC stratégie", Author: "Ezekiel", Content: "Comment battre le Roi Liche ?"},
}

var sseClients = make(map[chan Discussion]struct{})
var sseClientsMu sync.Mutex

func render(w http.ResponseWriter, page string, data interface{}) {
	tpl := template.Must(template.ParseFiles("templates/" + page))
	tpl.Execute(w, data)
}

func broadcastNewDiscussion(d Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()

	for ch := range sseClients {
		select {
		case ch <- d:
		default:
		}
	}
}

func addSSEClient(ch chan Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	sseClients[ch] = struct{}{}
}

func removeSSEClient(ch chan Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	delete(sseClients, ch)
}

func Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan Discussion, 1)
	addSSEClient(ch)
	defer removeSSEClient(ch)

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case d := <-ch:
			data, err := json.Marshal(d)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("event: discussion\n"))
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	}
}

func getDiscussions() []Discussion {
	discussions, err := GetDiscussionsFromDB()
	if err != nil || len(discussions) == 0 {
		return defaultDiscussions
	}
	return discussions
}

func Home(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Query().Get("message")
	render(w, "index.html", HomePageData{
		Discussions: getDiscussions(),
		Message:     message,
	})
}

func Profil(w http.ResponseWriter, r *http.Request) {
	userResult, err := getAuthenticatedUser(w, r)
	if err != nil || userResult.User.Email == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	username := GetUsernameByID(userResult.User.ID)
	if username == "" {
		username = GetUsernameByEmail(userResult.User.Email)
	}

	render(w, "profil.html", ProfilePageData{
		Email:    userResult.User.Email,
		ID:       userResult.User.ID,
		Username: username,
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	render(w, "Login.html", nil)
}

func Register(w http.ResponseWriter, r *http.Request) {
	render(w, "register.html", nil)
}

func Create(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		title := r.FormValue("title")
		author := r.FormValue("author")
		if author == "" {
			author = "Invité"
		}
		content := r.FormValue("content")

		if title == "" || content == "" {
			render(w, "create.html", CreatePageData{Error: "Titre et message requis."})
			return
		}

		newDiscussion, err := InsertDiscussion(title, author, content)
		if err != nil {
			render(w, "create.html", CreatePageData{Error: "Impossible d'enregistrer la discussion."})
			return
		}

		broadcastNewDiscussion(newDiscussion)
		http.Redirect(w, r, "/?message="+url.QueryEscape("Discussion publiée !"), http.StatusSeeOther)
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
