package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"github.com/serlenario/caching-webserver/internal/config"
	"github.com/serlenario/caching-webserver/internal/models"
	"github.com/serlenario/caching-webserver/internal/storage"
	"github.com/serlenario/caching-webserver/internal/utils"
	"net/http"
)

func Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
		Login string `json:"login"`
		Pswd  string `json:"pswd"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error": {"code": 400, "text": "Invalid parameters"}}`, http.StatusBadRequest)
		return
	}

	if input.Token != config.Config.AdminToken {
		http.Error(w, `{"error": {"code": 401, "text": "Unauthorized"}}`, http.StatusUnauthorized)
		return
	}

	if !utils.IsValidLogin(input.Login) || !utils.IsValidPassword(input.Pswd) {
		http.Error(w, `{"error": {"code": 400, "text": "Invalid parameters"}}`, http.StatusBadRequest)
		return
	}

	hashedPassword, err := utils.HashPassword(input.Pswd)
	if err != nil {
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	user := models.User{
		Login:    input.Login,
		Password: hashedPassword,
	}

	db := storage.GetDB()
	err = db.QueryRow("INSERT INTO users (login, password) VALUES ($1, $2) RETURNING id", user.Login, user.Password).Scan(&user.ID)
	if err != nil {
		if err.Error() == `pq: duplicate key value violates unique constraint "users_login_key"` {
			http.Error(w, `{"error": {"code": 400, "text": "Login already exists"}}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"response": map[string]string{
			"login": user.Login,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func Authenticate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Login string `json:"login"`
		Pswd  string `json:"pswd"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error": {"code": 400, "text": "Invalid parameters"}}`, http.StatusBadRequest)
		return
	}

	db := storage.GetDB()
	var user models.User
	err := db.QueryRow("SELECT id, password FROM users WHERE login=$1", input.Login).Scan(&user.ID, &user.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": {"code": 401, "text": "Unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	if !utils.CheckPasswordHash(input.Pswd, user.Password) {
		http.Error(w, `{"error": {"code": 401, "text": "Unauthorized"}}`, http.StatusUnauthorized)
		return
	}

	token, err := utils.GenerateToken()
	if err != nil {
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	storage.GetCache().Set(token, user.ID, cache.DefaultExpiration)

	response := map[string]interface{}{
		"response": map[string]string{
			"token": token,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func Logout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	storage.GetCache().Delete(token)

	response := map[string]interface{}{
		"response": map[string]bool{
			token: true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
