package main

import (
	"log"
	"net/http"
)

func main() {
	store := NewStore()
	// sample thread
	store.CreateThread("Bienvenue sur le forum WoW", "Admin", "Premier message de bienvenue")

	mux := NewServer(store)

	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
