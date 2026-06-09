package main

import (
	"fmt"
	"net/http"
)

func main() {
	router := newRouter()

	fmt.Println("http://localhost:8080")
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		fmt.Println("Erreur serveur:", err)
	}
}
