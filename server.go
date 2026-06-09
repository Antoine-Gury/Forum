package main

import (
	handlers "forum/src/go"
	"net/http"
)

func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	mux.HandleFunc("/", handlers.Home)
	mux.HandleFunc("/index.html", handlers.Home)
	mux.HandleFunc("/profil", handlers.Profil)
	mux.HandleFunc("/profil.html", handlers.Profil)
	mux.HandleFunc("/create", handlers.Create)
	mux.HandleFunc("/discussion", handlers.DiscussionPage)
	return mux
}
