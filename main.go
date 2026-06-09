package main

import (
	"bufio"
	"fmt"
	handlers "forum/src/go"
	"net/http"
	"os"
	"strings"
)

func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func main() {
	loadEnvFile()

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.Handle("/src/", http.StripPrefix("/src/", http.FileServer(http.Dir("src"))))

	if err := handlers.InitDB(); err != nil {
		fmt.Println("Erreur initialisation DB:", err)
	} else {
		defer handlers.CloseDB()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handlers.Home)
	http.HandleFunc("/index.html", handlers.Home)
	http.HandleFunc("/profil", handlers.Profil)
	http.HandleFunc("/profil.html", handlers.Profil)
	http.HandleFunc("/login", handlers.Login)
	http.HandleFunc("/login.html", handlers.Login)
	http.HandleFunc("/register", handlers.Register)
	http.HandleFunc("/register.html", handlers.Register)
	http.HandleFunc("/forgot", handlers.ForgotPassword)
	http.HandleFunc("/forget", handlers.ForgotPassword)
	http.HandleFunc("/auth/login", handlers.LoginHandler)
	http.HandleFunc("/auth/register", handlers.RegisterHandler)
	http.HandleFunc("/auth/forgot", handlers.ForgotHandler)
	http.HandleFunc("/logout", handlers.LogoutHandler)
	http.HandleFunc("/create", handlers.Create)
	http.HandleFunc("/discussion", handlers.DiscussionPage)

	fmt.Printf("http://localhost:%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Println("Erreur serveur:", err)
	}
}
