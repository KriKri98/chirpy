package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/KriKri98/chirpy/internal/auth"
	"github.com/KriKri98/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	jwtSecret := os.Getenv("JWT_SECRET")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Print(err)
		return
	}
	dbQueries := database.New(db)
	mux := http.NewServeMux()
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
		jwtSecret:      jwtSecret,
	}
	apiCfg.fileserverHits.Store(0)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", handlerReadinessEndpoint)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerHits)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerPostChirps)
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGetChirp)
	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateUser)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.handlerDeleteChirpByID)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.handlerUpgradeUser)
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	s.ListenAndServe()

}

func handlerReadinessEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtSecret      string
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Created_At   time.Time `json:"created_at"`
	Updated_At   time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

type Chirp struct {
	ID         uuid.UUID `json:"id"`
	Created_At time.Time `json:"created_at"`
	Updated_At time.Time `json:"updated_at"`
	Body       string    `json:"body"`
	User_ID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerHits(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	hits := cfg.fileserverHits.Load()
	output := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, hits)
	w.Write([]byte(output))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, 403, "Forbidden")
	}
	cfg.db.Reset(r.Context())
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)

}

func (apiCfg *apiConfig) handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
	}

	user, err := auth.ValidateJWT(token, apiCfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	chirp := database.CreateChirpParams{
		Body:   cleanChirp(params.Body),
		UserID: user,
	}

	createdChirp, err := apiCfg.db.CreateChirp(r.Context(), chirp)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	respondWithJSON(w, 201, Chirp{
		ID:         createdChirp.ID,
		Created_At: createdChirp.CreatedAt,
		Updated_At: createdChirp.UpdatedAt,
		Body:       createdChirp.Body,
		User_ID:    createdChirp.UserID,
	})

}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type returnVals struct {
		Error string `json:"error"`
	}

	respBody := returnVals{
		Error: msg,
	}
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {

	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func cleanChirp(msg string) string {
	splitString := strings.Split(msg, " ")
	for i, word := range splitString {
		lowerWord := strings.ToLower(word)
		if lowerWord == "kerfuffle" || lowerWord == "sharbert" || lowerWord == "fornax" {
			splitString[i] = "****"
		}
	}

	return strings.Join(splitString, " ")
}

func (apiCfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
	}

	userParams := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	user, err := apiCfg.db.CreateUser(r.Context(), userParams)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	resp := User{
		ID:          user.ID,
		Created_At:  user.CreatedAt,
		Updated_At:  user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	respondWithJSON(w, 201, resp)

}

func (apiCfg *apiConfig) handlerGetAllChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := apiCfg.db.GetAllChrips(r.Context())
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	allChirps := []Chirp{}
	for _, chirp := range chirps {
		allChirps = append(allChirps, Chirp{
			ID:         chirp.ID,
			Created_At: chirp.CreatedAt,
			Updated_At: chirp.UpdatedAt,
			Body:       chirp.Body,
			User_ID:    chirp.UserID,
		})
	}

	respondWithJSON(w, 200, allChirps)
}

func (apiCfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	chirpUUID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}
	chirp, err := apiCfg.db.GetChrip(r.Context(), chirpUUID)
	if err != nil {
		respondWithError(w, 404, err.Error())
		return
	}

	responseChirp := Chirp{
		ID:         chirp.ID,
		Created_At: chirp.CreatedAt,
		Updated_At: chirp.UpdatedAt,
		Body:       chirp.Body,
		User_ID:    chirp.UserID,
	}

	respondWithJSON(w, 200, responseChirp)
}

func (apiCfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	user, err := apiCfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	login, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil || login == false {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	expires := time.Duration(time.Second * 3600)

	token, err := auth.MakeJWT(user.ID, apiCfg.jwtSecret, expires)

	refreshToken := auth.MakeRefreshToken()

	refreshTokenStored, err := apiCfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
	})

	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	returnUser := User{
		ID:           user.ID,
		Created_At:   user.CreatedAt,
		Updated_At:   user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshTokenStored.Token,
		IsChirpyRed:  user.IsChirpyRed,
	}

	respondWithJSON(w, 200, returnUser)

}

func (apiCfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	storedRefreshToken, err := apiCfg.db.GetToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	if storedRefreshToken.RevokedAt.Valid == true || time.Now().UTC().Compare(storedRefreshToken.ExpiresAt) > 0 {
		respondWithError(w, 401, "refresh token expired or revoked")
		return
	}

	token, err := auth.MakeJWT(storedRefreshToken.UserID, apiCfg.jwtSecret, time.Duration(time.Second*3600))
	if err != nil {
		respondWithError(w, 500, err.Error())
	}

	type Token struct {
		Token string `json:"token"`
	}

	respondWithJSON(w, 200, Token{Token: token})

}

func (apiCfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	err = apiCfg.db.RevokeToken(r.Context(), token)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}
	respondWithJSON(w, 204, "")

}

func (apiCfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	acessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	userID, err := auth.ValidateJWT(acessToken, apiCfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	userToUpdate := database.UpdateUserByIDParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
		ID:             userID,
	}

	user, err := apiCfg.db.UpdateUserByID(r.Context(), userToUpdate)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	respondWithJSON(w, 200, User{ID: user.ID, Email: user.Email, Created_At: user.CreatedAt, Updated_At: user.UpdatedAt, IsChirpyRed: user.IsChirpyRed})
}

func (apiCfg *apiConfig) handlerDeleteChirpByID(w http.ResponseWriter, r *http.Request) {
	chirpUUID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	acessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	userID, err := auth.ValidateJWT(acessToken, apiCfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	chirp, err := apiCfg.db.GetChrip(r.Context(), chirpUUID)
	if err != nil {
		respondWithError(w, 404, err.Error())
		return
	}

	if chirp.UserID != userID {
		respondWithError(w, 403, "user not the author of chirp")
		return
	}

	err = apiCfg.db.DeleteChirpByID(r.Context(), chirpUUID)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	respondWithJSON(w, 204, "")

}

func (apiCfg *apiConfig) handlerUpgradeUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	if params.Event != "user.upgraded" {
		respondWithJSON(w, 204, "")
		return
	}

	err = apiCfg.db.UpgradeUserRed(r.Context(), params.Data.UserID)
	if err != nil {
		respondWithError(w, 404, err.Error())
		return
	}

	respondWithJSON(w, 204, "")
}
