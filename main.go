package main

import (
	"fmt"
	"net/http"

	server "forum/src/go"
)

func main() {
	router := server.NewRouter()

	fmt.Println("http://localhost:8080")
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		fmt.Println("Erreur serveur:", err)
	}
}
