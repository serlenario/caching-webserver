package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"github.com/serlenario/caching-webserver/internal/models"
	"github.com/serlenario/caching-webserver/internal/storage"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func UploadDocument(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, `{"error": {"code": 400, "text": "Invalid parameters"}}`, http.StatusBadRequest)
		return
	}

	metaData := r.FormValue("meta")
	if metaData == "" {
		http.Error(w, `{"error": {"code": 400, "text": "Missing metadata"}}`, http.StatusBadRequest)
		return
	}

	var meta struct {
		Name   string   `json:"name"`
		File   bool     `json:"file"`
		Public bool     `json:"public"`
		Token  string   `json:"token"`
		MIME   string   `json:"mime"`
		Grant  []string `json:"grant"`
	}
	if err := json.Unmarshal([]byte(metaData), &meta); err != nil {
		http.Error(w, `{"error": {"code": 400, "text": "Invalid metadata"}}`, http.StatusBadRequest)
		return
	}

	doc := models.Document{
		ID:        uuid.New().String(),
		OwnerID:   userID,
		Name:      meta.Name,
		MIME:      meta.MIME,
		File:      meta.File,
		Public:    meta.Public,
		Grant:     meta.Grant,
		CreatedAt: time.Now(),
	}

	if meta.File {
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error": {"code": 400, "text": "File not found"}}`, http.StatusBadRequest)
			return
		}

		defer func(file multipart.File) {
			err := file.Close()
			if err != nil {
				log.Printf("Error while closing the file: %v", err)
			}
		}(file)

		doc.Data, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, `{"error": {"code": 500, "text": "File reading error"}}`, http.StatusInternalServerError)
			return
		}
	} else {
		jsonData := r.FormValue("json")
		if jsonData == "" {
			http.Error(w, `{"error": {"code": 400, "text": "Missing JSON data"}}`, http.StatusBadRequest)
			return
		}
		doc.Data = []byte(jsonData)
	}

	db := storage.GetDB()
	_, err = db.Exec(
		`INSERT INTO documents (id, owner_id, name, mime, file, public, created_at, data, access_grant)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		doc.ID, doc.OwnerID, doc.Name, doc.MIME, doc.File, doc.Public, doc.CreatedAt, doc.Data, strings.Join(doc.Grant, ","),
	)
	if err != nil {
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	storage.GetCache().Delete("docs_list_" + string(rune(userID)))

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"json": json.RawMessage(doc.Data),
			"file": doc.Name,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func GetDocuments(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	query := r.URL.Query()
	login := query.Get("login")
	key := query.Get("key")
	value := query.Get("value")
	limitStr := query.Get("limit")

	limit, _ := strconv.Atoi(limitStr)
	if limit == 0 {
		limit = 10
	}

	cacheKey := "docs_list_" + strconv.Itoa(userID)
	if login != "" {
		cacheKey += "_login_" + login
	}
	if key != "" && value != "" {
		cacheKey += "_filter_" + key + "_" + value
	}

	if data, found := storage.GetCache().Get(cacheKey); found {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "Failed to encode cache data", http.StatusInternalServerError)
			return
		}

		return
	}

	db := storage.GetDB()
	var rows *sql.Rows
	var err error

	if login != "" {
		var targetUserID int
		err = db.QueryRow("SELECT id FROM users WHERE login=$1", login).Scan(&targetUserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, `{"error": {"code": 404, "text": "User not found"}}`, http.StatusNotFound)
				return
			}
			http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
			return
		}
		userID = targetUserID
	}

	queryStr := `SELECT id, name, mime, file, public, created_at, access_grant FROM documents WHERE owner_id=$1`
	args := []interface{}{userID}

	if key != "" && value != "" {
		queryStr += " AND " + key + "=$2"
		//args = append(args, value)
	}

	queryStr += " ORDER BY name, created_at LIMIT " + strconv.Itoa(limit)
	args = append(args, limit)

	rows, err = db.Query(queryStr, args...)
	if err != nil {
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error while closing rows: %v", err)
		}
	}(rows)

	var documents []map[string]interface{}

	for rows.Next() {
		var doc models.Document
		var grant string
		err := rows.Scan(&doc.ID, &doc.Name, &doc.MIME, &doc.File, &doc.Public, &doc.CreatedAt, &grant)
		if err != nil {
			http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
			return
		}
		doc.Grant = strings.Split(grant, ",")
		documents = append(documents, map[string]interface{}{
			"id":      doc.ID,
			"name":    doc.Name,
			"mime":    doc.MIME,
			"file":    doc.File,
			"public":  doc.Public,
			"created": doc.CreatedAt.Format("2006-01-02 15:04:05"),
			"grant":   doc.Grant,
		})
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"docs": documents,
		},
	}

	storage.GetCache().Set(cacheKey, response, cache.DefaultExpiration)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode cache data", http.StatusInternalServerError)
		return
	}

}

func GetDocument(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	vars := mux.Vars(r)
	docID := vars["id"]

	cacheKey := "doc_" + docID
	if data, found := storage.GetCache().Get(cacheKey); found {
		serveDocument(w, data.(models.Document))
		return
	}

	db := storage.GetDB()

	var doc models.Document
	var grant string

	err := db.QueryRow(
		`SELECT id, owner_id, name, mime, file, public, created_at, data, access_grant
        FROM documents WHERE id=$1`, docID,
	).Scan(&doc.ID, &doc.OwnerID, &doc.Name, &doc.MIME, &doc.File, &doc.Public, &doc.CreatedAt, &doc.Data, &grant)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": {"code": 404, "text": "Document not found"}}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	doc.Grant = strings.Split(grant, ",")

	if doc.OwnerID != userID && !doc.Public && !contains(doc.Grant, getUserLoginByID(userID)) {
		http.Error(w, `{"error": {"code": 403, "text": "Access denied"}}`, http.StatusForbidden)
		return
	}

	storage.GetCache().Set(cacheKey, doc, cache.DefaultExpiration)
	serveDocument(w, doc)
}

func serveDocument(w http.ResponseWriter, doc models.Document) {
	if doc.File {
		w.Header().Set("Content-Type", doc.MIME)

		if _, err := w.Write(doc.Data); err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
			return
		}
	} else {
		response := map[string]interface{}{
			"data": json.RawMessage(doc.Data),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getUserLoginByID(userID int) string {
	db := storage.GetDB()
	var login string
	err := db.QueryRow("SELECT login FROM users WHERE id=$1", userID).Scan(&login)
	if err != nil {
		return ""
	}
	return login
}

func DeleteDocument(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int)
	vars := mux.Vars(r)
	docID := vars["id"]

	db := storage.GetDB()

	var ownerID int

	err := db.QueryRow("SELECT owner_id FROM documents WHERE id=$1", docID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": {"code": 404, "text": "Document not found"}}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	if ownerID != userID {
		http.Error(w, `{"error": {"code": 403, "text": "Access denied"}}`, http.StatusForbidden)
		return
	}

	_, err = db.Exec("DELETE FROM documents WHERE id=$1", docID)
	if err != nil {
		http.Error(w, `{"error": {"code": 500, "text": "Server error"}}`, http.StatusInternalServerError)
		return
	}

	storage.GetCache().Delete("doc_" + docID)
	storage.GetCache().Delete("docs_list_" + strconv.Itoa(userID))

	response := map[string]interface{}{
		"response": map[string]bool{
			docID: true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode cache data", http.StatusInternalServerError)
		return
	}

}
