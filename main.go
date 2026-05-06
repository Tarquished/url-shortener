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

	_ "url-shortener/docs" // <-- GANTI "url-shortener" sesuai module name di go.mod kamu

	"github.com/golang-jwt/jwt/v5"
	httpSwagger "github.com/swaggo/http-swagger"
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
	URL string `json:"url" example:"https://example.com/very-long-url"`
}

type ResponseError struct {
	Error string `json:"error"`
}

type ResponPesan struct {
	Pesan string `json:"pesan"`
}

type ResponLogin struct {
	Pesan string `json:"pesan" example:"Berhasil login!"`
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
}

type GetURL struct {
	ID          int    `json:"id" example:"1"`
	OriginalURL string `json:"originalurl" example:"https://example.com/very-long-url"`
	ShortCode   string `json:"shortcode" example:"aBc123"`
	ClickCount  uint   `json:"clickcount" example:"5"`
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

// handlerCreateURL godoc
// @Summary      Buat short URL baru
// @Description  Membuat short URL dari URL panjang yang diberikan
// @Description  Short code di-generate random 6 karakter
// @Tags         URL
// @Accept       json
// @Produce      json
// @Param        request  body      inputData  true  "URL yang mau di-shorten"
// @Security     BearerAuth
// @Success      200      {object}  ResponPesan
// @Failure      400      {object}  ResponseError
// @Failure      405      {object}  ResponseError
// @Failure      500      {object}  ResponseError
// @Router       /create [post]
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponPesan{
		Pesan: "Berhasil menambahkan url ke database",
	})
}

// handlerUpdateURL godoc
// @Summary      Update URL berdasarkan ID
// @Description  Mengubah original URL dari short URL yang sudah ada
// @Tags         URL
// @Accept       json
// @Produce      json
// @Param        id       query     int        true  "ID URL"
// @Param        request  body      inputData  true  "URL baru"
// @Security     BearerAuth
// @Success      200      {object}  ResponPesan
// @Failure      400      {object}  ResponseError
// @Failure      405      {object}  ResponseError
// @Failure      500      {object}  ResponseError
// @Router       /update [put]
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
	json.NewEncoder(w).Encode(ResponPesan{
		Pesan: "URL berhasil diupdate",
	})
}

// handlerHapusURL godoc
// @Summary      Hapus URL berdasarkan ID
// @Description  Menghapus short URL dari database
// @Description  Diverifikasi melalui JWT untuk mengecek kepemilikan
// @Tags         URL
// @Produce      json
// @Param        id  query  int  true  "ID URL"
// @Security     BearerAuth
// @Success      200  {object}  ResponPesan
// @Failure      400  {object}  ResponseError
// @Failure      405  {object}  ResponseError
// @Failure      500  {object}  ResponseError
// @Router       /delete [delete]
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
	json.NewEncoder(w).Encode(ResponPesan{
		Pesan: "URL berhasil dihapus",
	})
}

// handlerRegister godoc
// @Summary      Register user baru
// @Description  Mendaftarkan user baru dengan username dan password
// @Description  Password di-hash menggunakan bcrypt
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      InputAuth    true  "Username dan password"
// @Success      200      {object}  ResponPesan
// @Failure      400      {object}  ResponseError
// @Failure      405      {object}  ResponseError
// @Router       /register [post]
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponPesan{
		Pesan: "Berhasil menambahkan username ke database",
	})
}

// handlerLogin godoc
// @Summary      Login user
// @Description  Login dengan username dan password, mendapatkan JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      InputAuth    true  "Username dan password"
// @Success      200      {object}  ResponLogin
// @Failure      400      {object}  ResponseError
// @Failure      405      {object}  ResponseError
// @Router       /login [post]
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponLogin{
		Pesan: "Berhasil login!",
		Token: tokenString,
	})
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

	if code == "login" || code == "register" || code == "create" || code == "update" || code == "delete" || code == "urls" || strings.HasPrefix(code, "swagger") {
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

// handlerUrls godoc
// @Summary      Lihat semua URL milik user
// @Description  Menampilkan seluruh short URL yang dimiliki user
// @Description  Termasuk original URL, short code, dan click count
// @Tags         URL
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   GetURL
// @Failure      405  {object}  ResponseError
// @Router       /urls [get]
func handlerUrls(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendError(w, "method harus GET", 405)
		return
	}

	userID := getUserID(r)
	var urls []urlData
	db.Where("user_id = ?", userID).Find(&urls)

	var hasil []GetURL
	for _, v := range urls {
		hasil = append(hasil, GetURL{
			ID:          int(v.ID),
			OriginalURL: v.OriginalURL,
			ShortCode:   v.ShortCode,
			ClickCount:  v.ClickCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hasil)
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

// @title           URL Shortener API
// @version         1.0
// @description     REST API untuk mempersingkat URL dengan JWT authentication
// @description     Fitur: shorten URL, click counter, redirect, user ownership

// @contact.name    Jason
// @contact.url     https://github.com/Tarquished

// @host            url-shortener-production-d0ce.up.railway.app
// @schemes			https
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Masukkan token dengan format: Bearer <token>

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
	http.HandleFunc("/urls", recoveryMiddleware(authMiddleware(handlerUrls)))
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	http.HandleFunc("/", recoveryMiddleware(handlerGoToURL))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server jalan di port", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
