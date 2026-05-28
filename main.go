package main

import (
	"html/template"
	"log"
	"net/http"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

func main() {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { render(w, "home") })
	mux.HandleFunc("/forum", func(w http.ResponseWriter, r *http.Request) { render(w, "forum") })
	mux.HandleFunc("/compte", func(w http.ResponseWriter, r *http.Request) { render(w, "compte") })

	log.Println("Serveur démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func render(w http.ResponseWriter, page string) {
	err := templates.ExecuteTemplate(w, "layout.html", map[string]string{"Page": page})
	if err != nil {
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		log.Println(err)
	}
}
