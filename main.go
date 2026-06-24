package main

import (
	"bootdev-go-server/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	User_id   uuid.UUID `json:"user_id"`
}

func initDB() (*database.Queries, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB URL NOT SET")
	}

	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error opening database: %s", err)
	}

	return database.New(dbConn), nil

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})

}

func (cfg *apiConfig) handleReset(w http.ResponseWriter, r *http.Request) {
	//cfg.fileserverHits.Store(0)
	//w.WriteHeader(http.StatusOK)
	//w.Write([]byte("Hits reset to 0"))
	platformEnv := os.Getenv("PLATFORM")

	if platformEnv != "dev" {
		w.WriteHeader(403)
		return
	}

	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	w.WriteHeader(200)

}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	param := params{}
	err := decoder.Decode(&param)
	if err != nil {
		log.Printf("Faild Decoding")
		w.WriteHeader(500)
		return
	}

	defer r.Body.Close()

	createdUser, err := cfg.db.CreateUser(r.Context(), param.Email)
	if err != nil {
		log.Fatal(err)
		return
	}

	newUser := User{
		ID:        createdUser.ID,
		CreatedAt: createdUser.CreatedAt,
		UpdatedAt: createdUser.UpdatedAt,
		Email:     createdUser.Email,
	}

	data, err := json.Marshal(newUser)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("Server Error")
	}

	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)

}

func (cfg *apiConfig) handleMetric(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>
	`, cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) validateChirpMiddelware(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Body    string    `json:"body"`
		User_id uuid.UUID `json:"user_id"`
	}

	type cleanedString struct {
		Cleaned_string string `json:"cleaned_body"`
	}
	type isError struct {
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	param := params{}
	err := decoder.Decode(&param)
	if err != nil {
		log.Printf("Faild Decoding")
		w.WriteHeader(500)
		return
	}

	// if chrip is longer than 140 chars
	if len(param.Body) > 140 {
		respBody := isError{
			Error: "Chrip is too long",
		}

		data, err := json.Marshal(respBody)
		if err != nil {
			w.WriteHeader(500)
			log.Printf("Server Error")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(data)
		return
	}

	//process every word and find bad words [kerfuffle, sharbert, fornax]
	allWords := strings.Fields(param.Body)
	for i, word := range allWords {
		if strings.ToLower(word) == "kerfuffle" {
			allWords[i] = "****"
		} else if strings.ToLower(word) == "fornax" {
			allWords[i] = "****"
		} else if strings.ToLower(word) == "sharbert" {
			allWords[i] = "****"
		} else {
			continue
		}
	}

	// make word slice back to string
	completeString := strings.Join(allWords, " ")

	chi := database.CreateChirpParams{
		Body:   completeString,
		UserID: param.User_id,
	}

	createChirp, err := cfg.db.CreateChirp(r.Context(), chi)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	dbResponseChrip := Chirp{
		ID:        createChirp.ID,
		CreatedAt: createChirp.CreatedAt,
		UpdatedAt: createChirp.UpdatedAt,
		Body:      createChirp.Body,
		User_id:   createChirp.UserID,
	}

	data, err := json.Marshal(dbResponseChrip)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	w.WriteHeader(201)
	w.Write(data)

}

func main() {
	godotenv.Load()
	dbQueries, err := initDB()
	if err != nil {
		log.Fatal(err)
	}

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
	}

	const port = "8080"
	mux := http.NewServeMux()
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("POST /api/chirps", apiCfg.validateChirpMiddelware)

	mux.HandleFunc("POST /api/users", apiCfg.createUser)

	mux.HandleFunc("GET /admin/metrics", apiCfg.handleMetric)

	mux.HandleFunc("POST /admin/reset", apiCfg.handleReset)

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	log.Printf("Starting server on port: %s", port)
	log.Fatal(server.ListenAndServe())

}
