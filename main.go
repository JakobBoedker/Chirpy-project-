package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})

}

func (cfg *apiConfig) handleReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))

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

func validateChirpMiddelware(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Body string `json:"body"`
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

	cleanStringStruct := cleanedString{
		Cleaned_string: completeString,
	}

	dat, err := json.Marshal(cleanStringStruct)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
	return

}

func main() {

	apiCfg := &apiConfig{}

	const port = "8080"
	mux := http.NewServeMux()
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /admin/metrics", apiCfg.handleMetric)

	mux.HandleFunc("POST /admin/reset", apiCfg.handleReset)

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	mux.HandleFunc("POST /api/validate_chirp", validateChirpMiddelware)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	log.Printf("Starting server on port: %s", port)
	log.Fatal(server.ListenAndServe())

}
