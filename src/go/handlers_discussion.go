package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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

		// Récupérer le pseudo depuis la session si l'utilisateur est connecté
		author := "Invité"
		if userResult, err := getAuthenticatedUser(w, r); err == nil {
			if uname := GetUsernameByID(userResult.User.ID); uname != "" {
				author = uname
			} else if userResult.User.Email != "" {
				author = userResult.User.Email
			}
		}

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

func CreateReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userResult, err := getAuthenticatedUser(w, r)
	if err != nil || userResult.User.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	discID, err := strconv.Atoi(r.FormValue("discussion_id"))
	if err != nil {
		http.Error(w, "id invalide", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if strings.TrimSpace(content) == "" {
		http.Redirect(w, r, "/discussion?id="+strconv.Itoa(discID), http.StatusSeeOther)
		return
	}

	author, avatar := resolveAuthor(w, r)
	if _, err := InsertReply(discID, author, content, avatar); err != nil {
		http.Error(w, "erreur serveur", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/discussion?id="+strconv.Itoa(discID), http.StatusSeeOther)
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
