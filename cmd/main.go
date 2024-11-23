package main

import (
	"log"
	"net/http"

	"github.com/serlenario/caching-webserver/internal/config"
	"github.com/serlenario/caching-webserver/internal/handlers"
	"github.com/serlenario/caching-webserver/internal/middleware"
	"github.com/serlenario/caching-webserver/internal/storage"

	"github.com/gorilla/mux"
)

func main() {
	config.LoadConfig()

	err := storage.InitDB(config.Config.DatabaseURL)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)

	}
	defer storage.CloseDB()

	storage.InitCache()

	router := mux.NewRouter()

	router.HandleFunc("/api/register", handlers.Register).Methods("POST")
	router.HandleFunc("/api/auth", handlers.Authenticate).Methods("POST")

	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)
	api.HandleFunc("/docs", handlers.GetDocuments).Methods("GET", "HEAD")
	api.HandleFunc("/docs", handlers.UploadDocument).Methods("POST")
	api.HandleFunc("/docs/{id}", handlers.GetDocument).Methods("GET", "HEAD")
	api.HandleFunc("/docs/{id}", handlers.DeleteDocument).Methods("DELETE")
	api.HandleFunc("/auth/{token}", handlers.Logout).Methods("DELETE")

	log.Println("Server started on port 8080")

	err = http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatal("Error starting the server:", err)
	}
}
