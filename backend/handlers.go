package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

func NewServer(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { indexHandler(s, w, r) })
	mux.HandleFunc("/thread/new", func(w http.ResponseWriter, r *http.Request) { newThreadHandler(s, w, r) })
	mux.HandleFunc("/thread/create", func(w http.ResponseWriter, r *http.Request) { createThreadHandler(s, w, r) })
	mux.HandleFunc("/thread/view", func(w http.ResponseWriter, r *http.Request) { viewThreadHandler(s, w, r) })
	mux.HandleFunc("/thread/post", func(w http.ResponseWriter, r *http.Request) { postHandler(s, w, r) })
	return mux
}

func indexHandler(s *Store, w http.ResponseWriter, r *http.Request) {
	threads := s.ListThreads()
	err := templates.ExecuteTemplate(w, "layout.html", map[string]interface{}{"Threads": threads})
	if err != nil {
		log.Println("template error:", err)
	}
}

func newThreadHandler(s *Store, w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "layout.html", map[string]interface{}{"New": true})
	if err != nil {
		log.Println("template error:", err)
	}
}

func createThreadHandler(s *Store, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	title := r.FormValue("title")
	author := r.FormValue("author")
	content := r.FormValue("content")
	if title == "" || content == "" {
		http.Error(w, "Titre et message requis", http.StatusBadRequest)
		return
	}
	id := s.CreateThread(title, author, content)
	http.Redirect(w, r, "/thread/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func viewThreadHandler(s *Store, w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID invalide", http.StatusBadRequest)
		return
	}
	thread, err := s.GetThread(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = templates.ExecuteTemplate(w, "layout.html", map[string]interface{}{"Thread": thread})
	if err != nil {
		log.Println("template error:", err)
	}
}

func postHandler(s *Store, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	idStr := r.FormValue("thread_id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID invalide", http.StatusBadRequest)
		return
	}
	author := r.FormValue("author")
	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Message requis", http.StatusBadRequest)
		return
	}
	err = s.AddPost(id, author, content)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/thread/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}
