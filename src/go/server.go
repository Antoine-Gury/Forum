package server

import (
	"net/http"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	mux.HandleFunc("/", Home)
	mux.HandleFunc("/index.html", Home)
	mux.HandleFunc("/profil", Profil)
	mux.HandleFunc("/profil.html", Profil)
	mux.HandleFunc("/create", Create)
	mux.HandleFunc("/discussion", DiscussionPage)
	return mux
}
