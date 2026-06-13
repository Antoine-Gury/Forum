package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type Discussion struct {
	ID        int
	Title     string
	Author    string
	Content   string
	AvatarURL string
	Score     int
	UserVote  int
}

type HomePageData struct {
	Discussions []Discussion
	Message     string
}

type ProfilePageData struct {
	Email       string
	ID          string
	Username    string
	AvatarURL   string
	AvatarError string
	Error       string
}

type CreatePageData struct {
	Error  string
	Author string
}

var defaultDiscussions = []Discussion{
	{ID: 0, Title: "Raid ICC stratégie", Author: "Ezekiel", Content: "Comment battre le Roi Liche ?", AvatarURL: ""},
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

func resolveAuthor(w http.ResponseWriter, r *http.Request) (string, string) {
	userResult, err := getAuthenticatedUser(w, r)
	if err != nil {
		return "Invité", ""
	}

	email := userResult.User.Email
	userID := userResult.User.ID

	username := GetUsernameByEmail(email)
	if username == "" {
		username = GetUsernameByID(userID)
	}
	if username == "" && email != "" {
		for i, c := range email {
			if c == '@' {
				username = email[:i]
				break
			}
		}
	}
	if username == "" {
		username = "Invité"
	}

	avatarURL := GetAvatarURLByEmail(email)
	return username, avatarURL
}

func Create(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		title := r.FormValue("title")
		content := r.FormValue("content")

		if title == "" || content == "" {
			author, _ := resolveAuthor(w, r)
			render(w, "create.html", CreatePageData{Error: "Titre et message requis.", Author: author})
			return
		}

		author, avatarURL := resolveAuthor(w, r)
		fmt.Printf("[Create] title=%q author=%q avatar=%q\n", title, author, avatarURL)

		newDiscussion, err := InsertDiscussion(title, author, content, avatarURL)
		if err != nil {
			fmt.Printf("[Create] InsertDiscussion error: %v\n", err)
			render(w, "create.html", CreatePageData{Error: "Impossible d'enregistrer la discussion.", Author: author})
			return
		}

		broadcastNewDiscussion(newDiscussion)
		http.Redirect(w, r, "/?message="+url.QueryEscape("Discussion publiée !"), http.StatusSeeOther)
		return
	}

	author, _ := resolveAuthor(w, r)
	render(w, "create.html", CreatePageData{Author: author})
}

func DiscussionPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)
	userID := currentUserID(w, r)

	if discussion, err := GetDiscussionByID(id, userID); err == nil {
		render(w, "discussion.html", discussion)
		return
	}
	for _, d := range getDiscussions(userID) {
		if d.ID == id {
			render(w, "discussion.html", d)
			return
		}
	}
	http.NotFound(w, r)
}

type voteResponse struct {
	Score    int `json:"score"`
	UserVote int `json:"user_vote"`
}

func Vote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userResult, err := getAuthenticatedUser(w, r)
	if err != nil || userResult.User.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(r.FormValue("discussion_id"))
	if err != nil {
		http.Error(w, "id invalide", http.StatusBadRequest)
		return
	}

	value, err := strconv.Atoi(r.FormValue("value"))
	if err != nil || (value != -1 && value != 1) {
		http.Error(w, "valeur invalide", http.StatusBadRequest)
		return
	}

	current := GetUserVote(id, userResult.User.ID)
	newValue := value
	if current == value {
		newValue = 0
	}

	if _, err := UpsertVote(id, userResult.User.ID, newValue); err != nil {
		http.Error(w, "erreur serveur", http.StatusInternalServerError)
		return
	}

	redirectTo := r.FormValue("redirect")
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}
