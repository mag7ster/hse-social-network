package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	db            DBWrapper
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
)

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

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		Role:         "default",
	}

	err = db.CreateUser(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	err = db.CreateEmptyProfile(user.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered"})
}

func loginHandler(c *gin.Context) {
	var req struct {
		Login    string `json:"login" binding:"required,login"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	user, err := db.GetUserByLogin(req.Login)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	tokenString, expiration, err := GenerateToken(user)
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

	err = db.CreateSession(&session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func whoamiHandler(c *gin.Context) {
	_, claims, err := Authenticate(c)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := GetUserIdByClaims(claims)

	user, err := db.GetUserByUserId(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.PasswordHash = "not your business"

	c.JSON(http.StatusOK, user)
}

func logoutHandler(c *gin.Context) {
	_, claims, err := Authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := GetUserIdByClaims(claims)

	db.DeleteSessionByUserId(id)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func updateProfileHandler(c *gin.Context) {
	_, claims, err := Authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := GetUserIdByClaims(claims)

	profile, err := db.GetProfileByUserId(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	db.UpdateProfile(profile)
	c.JSON(http.StatusOK, gin.H{"message": "Profile updated"})
}

func getProfileHandler(c *gin.Context) {
	_, claims, err := Authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := GetUserIdByClaims(claims)

	profile, err := db.GetProfileByUserId(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func main() {
	log.Println("I am user service")

	port := flag.Int("port", 8090, "Порт сервиса")
	dbURL := flag.String("db", "", "URL БД")
	privateKeyPath := flag.String("private", "", "Приватный ключ")
	publicKeyPath := flag.String("public", "", "Публичный ключ")
	flag.Parse()

	db = InitDB(*dbURL)
	LoadRSAKeys(*privateKeyPath, *publicKeyPath)

	r := gin.Default()
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)
	r.POST("/logout", logoutHandler)
	r.GET("/whoami", whoamiHandler)
	r.PUT("/profile/update", updateProfileHandler)
	r.GET("/profile", getProfileHandler)

	r.Run(fmt.Sprintf(":%d", *port))
}
