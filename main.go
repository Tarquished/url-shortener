package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"context"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

type User struct {
	gorm.Model
	Username string `json:"username"`
	Password string `json:"password"`
}

type InputAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type urlData struct {
	gorm.Model
	UserID      uint   `json:"user_id"`
	OriginalURL string `json:"originalurl"`
	ShortCode   string `json:"shortcode"`
	ClickCount  uint   `json:"clickcount"`
}

type inputData struct {
	URL string `json:"url"`
}

type ResponseError struct {
	Error string `json:"error"`
}

func sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ResponseError{Error: message})
}

func generateShortCode() string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)
	for i := range code {
		code[i] = chars[rand.Intn(len(chars))]
	}
	return string(code)
}

func getUserID(r *http.Request) uint {
	claims := r.Context().Value("claims").(*jwt.MapClaims)
	userID := uint((*claims)["user_id"].(float64))
	return userID
}

func handlerCreateURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendError(w, "method harus POST", 405)
		return
	}

	var input inputData
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		sendError(w, "format JSON tidak vlaid", 400)
		return
	}

	if input.URL == "" {
		sendError(w, "url harus terisi", 400)
		return
	}
	userID := getUserID(r)
	urlCode := generateShortCode()

	result := db.Create(&urlData{
		UserID:      userID,
		OriginalURL: input.URL,
		ShortCode:   urlCode,
		ClickCount:  0,
	})

	if result.Error != nil {
		log.Printf("ERROR handlerURL - db.Create failed: %v, userID: %d", result.Error, userID)
		sendError(w, "gagal menyimpan data", 500)
		return
	}

	succesRespon := map[string]any{
		"pesan": "Berhasil menambahkan url ke database",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(succesRespon)
}

func handlerUpdateURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		sendError(w, "method harus PUT", 405)
		return
	}

	strID := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strID)
	if err != nil {
		sendError(w, "ID tidak valid", 400)
		return
	}
	if id == 0 {
		sendError(w, "ID tidak terdaftar", 400)
		return
	}

	var input inputData
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		sendError(w, "format JSON tidak vlaid", 400)
		return
	}

	userID := getUserID(r)

	if input.URL == "" {
		sendError(w, "url harus terisi", 400)
		return
	}
	result := db.Model(&urlData{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]any{
		"original_url": input.URL,
	})

	if result.Error != nil {
		log.Printf("ERROR handlerUpdateURL - db.Update failed: %v, userID: %d", result.Error, userID)
		sendError(w, "gagal mengupdate data", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pesan": "URL berhasil diupdate",
	})
}

func handlerHapusURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		sendError(w, "method harus DELETE", 405)
		return
	}

	strID := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strID)
	if err != nil {
		sendError(w, "ID tidak valid", 400)
		return
	}
	if id == 0 {
		sendError(w, "ID tidak terdaftar", 400)
		return
	}
	userID := getUserID(r)
	result := db.Where("id = ? AND user_id = ?", id, userID).Delete(&urlData{})
	if result.Error != nil {
		log.Printf("ERROR handlerHapusURL - db.Delete failed: %v, userID: %d", result.Error, userID)
		sendError(w, "gagal menghapus data", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pesan": "URL berhasil dihapus",
	})
}

func handlerRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendError(w, "method harus POST", 405)
		return
	}

	var input InputAuth
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		sendError(w, "format JSON tidak valid", 400)
		return
	}

	if input.Username == "" {
		sendError(w, "mohon isi username", 400)
		return
	}
	if input.Password == "" {
		sendError(w, "mohon isi password", 400)
		return
	}

	var user User

	results := db.Where("username = ?", input.Username).First(&user)
	if results.Error == nil {
		sendError(w, "username sudah ada", 400)
		return
	}
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 14)
	if err != nil {
		sendError(w, "error saat hashing password", 400)
		return
	}
	db.Create(&User{
		Username: input.Username,
		Password: string(hashPassword),
	})
	succesRespon := map[string]any{
		"pesan": "Berhasil menambahkan username ke database",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(succesRespon)
}

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendError(w, "method harus POST", 405)
		return
	}

	var input InputAuth
	err := json.NewDecoder(r.Body).Decode(&input)

	if err != nil {
		sendError(w, "format JSON salah", 400)
		return
	}
	if input.Username == "" {
		sendError(w, "mohon isi username", 400)
		return
	}
	if input.Password == "" {
		sendError(w, "mohon isi password", 400)
		return
	}

	var user User

	results := db.Where("username = ?", input.Username).First(&user)
	if results.Error != nil {
		sendError(w, "username belum ada", 400)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		sendError(w, "password salah", 400)
		return
	}

	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secretkey := os.Getenv("JWT_SECRET")
	if secretkey == "" {
		secretkey = "test1625jason34"
	}
	tokenString, err := token.SignedString([]byte(secretkey))

	if err != nil {
		sendError(w, "gagal generate token", 400)
		return
	}
	succesRespon := map[string]any{
		"pesan": "Berhasil login!",
		"token": tokenString,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(succesRespon)
}

func algoritma(t *jwt.Token) (interface{}, error) {
	secretkey := os.Getenv("JWT_SECRET")
	if secretkey == "" {
		secretkey = "test1625jason34"
	}
	return []byte(secretkey), nil
}

func verifyToken(r *http.Request) (*jwt.MapClaims, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims := &jwt.MapClaims{}

	responToken, err := jwt.ParseWithClaims(token, claims, algoritma)

	if err != nil {
		return nil, err
	}
	if !responToken.Valid {
		return nil, fmt.Errorf("token tidak valid")
	}
	return claims, nil
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := verifyToken(r)
		if err != nil {
			sendError(w, "tidak valid", 401)
			return
		}
		ctx := context.WithValue(r.Context(), "claims", claims)
		next(w, r.WithContext(ctx))
	}
}

func handlerGoToURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendError(w, "method harus GET", 405)
		return
	}

	code := r.URL.Path[1:]

	if code == "login" || code == "register" || code == "create" || code == "update" || code == "delete" {
		return
	}
	var temp urlData

	db.Where("short_code = ?", code).Find(&temp)
	if temp.ID == 0 {
		sendError(w, "url tidak ditemukan", 400)
		return
	}

	db.Model(&urlData{}).Where("short_code = ?", code).Update("click_count", gorm.Expr("click_count + 1"))

	http.Redirect(w, r, temp.OriginalURL, http.StatusFound)
}

func handlerUrls(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendError(w, "method harus GET", 405)
		return
	}

	userID := getUserID(r)
	var urls []urlData
	db.Where("user_id = ?", userID).Find(&urls)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}

func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("ERROR recoveryMiddlewar:%v", err)
				sendError(w, "terjadi kesalahan", 500)
			}
		}()
		next(w, r)
	}

}
func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=test162534 dbname=url_shortener port=5432 sslmode=disable"
	}

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("Gagal konek ke database:", err)
		return
	}

	db.AutoMigrate(&urlData{})
	db.AutoMigrate(&User{})
	http.HandleFunc("/register", recoveryMiddleware(handlerRegister))
	http.HandleFunc("/login", recoveryMiddleware(handlerLogin))
	http.HandleFunc("/create", recoveryMiddleware(authMiddleware(handlerCreateURL)))
	http.HandleFunc("/update", recoveryMiddleware(authMiddleware(handlerUpdateURL)))
	http.HandleFunc("/delete", recoveryMiddleware(authMiddleware(handlerHapusURL)))
	http.HandleFunc("/", recoveryMiddleware(handlerGoToURL))
	http.HandleFunc("/urls", recoveryMiddleware(authMiddleware(handlerUrls)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server jalan di port", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
