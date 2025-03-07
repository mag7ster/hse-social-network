package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Модели

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Username     string    `gorm:"unique;not null"`
	Email        string    `gorm:"unique;not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	Role         string    `gorm:"not null"`
}

type UserProfile struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;unique"`
	FirstName string    `gorm:"size:100"`
	LastName  string    `gorm:"size:100"`
	Bio       string    `gorm:"size:500"`
	AvatarURL string    `gorm:"size:255"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type UserSession struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null"`
	Token     string    `gorm:"not null;unique"`
	LoginAt   time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time `gorm:"not null"`
	IPAddress string    `gorm:"size:45"`
}

// Глобальные переменные
var (
	db            *gorm.DB
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
)

func initDB(dbURL string) *gorm.DB {
	database, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	if err := database.AutoMigrate(&User{}, &UserProfile{}, &UserSession{}); err != nil {
		log.Fatalf("Ошибка миграции БД: %v", err)
	}
	return database
}

func loadRSAKeys(privatePath, publicPath string) {
	privData, err := os.ReadFile(privatePath)
	if err != nil {
		log.Fatalf("Ошибка чтения приватного ключа: %v", err)
	}
	rsaPrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM(privData)
	if err != nil {
		log.Fatalf("Ошибка парсинга приватного ключа: %v", err)
	}

	pubData, err := os.ReadFile(publicPath)
	if err != nil {
		log.Fatalf("Ошибка чтения публичного ключа: %v", err)
	}
	rsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(pubData)
	if err != nil {
		log.Fatalf("Ошибка парсинга публичного ключа: %v", err)
	}
}

func generateToken(user User) (string, time.Time, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     expirationTime.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(rsaPrivateKey)
	return tokenString, expirationTime, err
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func registerHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		Role:         "user",
	}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	userProfile := UserProfile{
		UserID:    user.ID,
		FirstName: "",
		LastName:  "",
		Bio:       "",
		AvatarURL: "",
	}

	if err := db.Create(&userProfile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered"})
}

func loginHandler(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	var user User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !checkPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	tokenString, expiration, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	session := UserSession{
		UserID:    user.ID,
		Token:     tokenString,
		LoginAt:   time.Now(),
		ExpiresAt: expiration,
		IPAddress: c.ClientIP(),
	}
	if err := db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func whoamiHandler(c *gin.Context) {
	_, claims, err := authenticate(c)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user User
	if err := db.Where("id = ?", claims["user_id"]).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.PasswordHash = "permission denied"

	c.JSON(http.StatusOK, user)
}

func logoutHandler(c *gin.Context) {
	_, claims, err := authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	db.Where("user_id = ?", claims["user_id"]).Delete(&UserSession{})
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func updateProfileHandler(c *gin.Context) {
	_, claims, err := authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var profile UserProfile
	if err := db.Where("user_id = ?", claims["user_id"]).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	db.Save(&profile)
	c.JSON(http.StatusOK, gin.H{"message": "Profile updated"})
}

func getProfileHandler(c *gin.Context) {
	_, claims, err := authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var profile UserProfile
	if err := db.Where("user_id = ?", claims["user_id"]).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func authenticate(c *gin.Context) (*jwt.Token, jwt.MapClaims, error) {
	tokenString := c.GetHeader("Authorization")
	fmt.Println(tokenString)
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return rsaPublicKey, nil
	})
	if err != nil {
		return nil, nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, nil, fmt.Errorf("invalid token")
	}
	return token, claims, nil
}

func main() {
	port := flag.Int("port", 8090, "Порт сервиса")
	dbURL := flag.String("db", "", "URL БД")
	privateKeyPath := flag.String("private", "", "Приватный ключ")
	publicKeyPath := flag.String("public", "", "Публичный ключ")
	flag.Parse()

	db = initDB(*dbURL)
	loadRSAKeys(*privateKeyPath, *publicKeyPath)

	r := gin.Default()
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)
	r.POST("/logout", logoutHandler)
	r.GET("/whoami", whoamiHandler)
	r.PUT("/profile/update", updateProfileHandler)
	r.GET("/profile", getProfileHandler)

	r.Run(fmt.Sprintf(":%d", *port))
}
