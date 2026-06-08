package main

import (
	"fmt"
	handlers "forum/src/go"
	"net/http"
)

func main() {
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	http.HandleFunc("/", handlers.Home)
	http.HandleFunc("/profil", handlers.Profil)
	http.HandleFunc("/create", handlers.Create)
	http.HandleFunc("/discussion", handlers.DiscussionPage)

	fmt.Println("http://localhost:8080 ✅")
	http.ListenAndServe(":8080", nil)
}
