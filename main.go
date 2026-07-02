package main

import (
	"bootdev-go-server/internal/auth"
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
	secret         string
}

type User struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"password"`
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
		log.Fatalf("Error opening database: %s", err)
	}

	return database.New(dbConn), nil

}

func (cfg *apiConfig) getSingleChirp(w http.ResponseWriter, r *http.Request) {
	chirpIDString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(500)
	}

	single, err := cfg.db.GetOneChirp(r.Context(), chirpID)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	newChirp := Chirp{
		ID:        single.ID,
		CreatedAt: single.CreatedAt,
		UpdatedAt: single.UpdatedAt,
		Body:      single.Body,
		User_id:   single.UserID,
	}

	data, err := json.Marshal(newChirp)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
	}

	w.WriteHeader(200)
	w.Write(data)

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

func (cfg *apiConfig) getChrips(w http.ResponseWriter, r *http.Request) {
	allItems, err := cfg.db.GetAllChrips(r.Context())
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	var formattedChrips []Chirp

	for _, chirp := range allItems {
		newChirp := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			User_id:   chirp.UserID,
		}
		formattedChrips = append(formattedChrips, newChirp)

	}

	data, err := json.Marshal(formattedChrips)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	param := params{}
	err := decoder.Decode(&param)
	if err != nil {
		log.Printf("Faild Decoding")
		w.WriteHeader(500)
		return
	}

	hashThatPassword, err := auth.HashPassword(param.Password)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	newUser := database.CreateUserParams{
		HashedPassword: hashThatPassword,
		Email:          param.Email,
	}

	defer r.Body.Close()

	createdUser, err := cfg.db.CreateUser(r.Context(), newUser)
	if err != nil {
		log.Fatal(err)
		return
	}

	type userWithoutPass struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	newUser1 := userWithoutPass{
		ID:        createdUser.ID,
		CreatedAt: createdUser.CreatedAt,
		UpdatedAt: createdUser.UpdatedAt,
		Email:     createdUser.Email,
	}

	data, err := json.Marshal(newUser1)
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

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Password   string `json:"password"`
		Email      string `json:"email"`
		Expires_in int    `json:"expires_in_seconds,omitempty"`
	}

	type userWithoutPass struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
		Token     string    `json:"token"`
	}
	decode := json.NewDecoder(r.Body)
	param := params{}
	err := decode.Decode(&param)
	if err != nil {
		log.Printf("failed decoding")
		w.WriteHeader(500)
		return
	}

	loggedInUser, err := cfg.db.LogUserIn(r.Context(), param.Email)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	match, err := auth.CheckPasswordHash(param.Password, loggedInUser.HashedPassword)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}

	if !match {
		w.WriteHeader(401)
		return
	}
	var timeExpiresIn time.Duration

	// make JWT token
	if param.Expires_in == 0 || param.Expires_in >= 3600 {
		timeExpiresIn = time.Duration(time.Hour)
	} else {
		timeExpiresIn = time.Duration(param.Expires_in) * time.Second
	}
	jwtToken, err := auth.MakeJWT(loggedInUser.ID, cfg.secret, timeExpiresIn)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
	}

	newlogin := userWithoutPass{
		ID:        loggedInUser.ID,
		CreatedAt: loggedInUser.CreatedAt,
		UpdatedAt: loggedInUser.UpdatedAt,
		Email:     loggedInUser.Email,
		Token:     jwtToken,
	}
	data, err := json.Marshal(newlogin)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
		return
	}
	w.WriteHeader(200)
	w.Write(data)

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

	getToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(500)
		log.Fatal(err)
	}

	checkToken, err := auth.ValidateJWT(getToken, cfg.secret)
	if err != nil {
		w.WriteHeader(401)
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
		UserID: checkToken,
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
		secret:         os.Getenv("JWT_SECRET"),
	}

	const port = "8080"
	mux := http.NewServeMux()
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("POST /api/chirps", apiCfg.validateChirpMiddelware)

	mux.HandleFunc("GET /api/chirps", apiCfg.getChrips)

	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getSingleChirp)

	mux.HandleFunc("POST /api/users", apiCfg.createUser)

	mux.HandleFunc("POST /api/login", apiCfg.loginUser)

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
