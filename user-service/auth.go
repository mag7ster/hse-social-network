package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func LoadRSAKeys(privatePath, publicPath string) {
	privData, err := os.ReadFile(privatePath)
	if err != nil {
		log.Fatalf("Reading private key error: %v", err)
	}
	rsaPrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM(privData)
	if err != nil {
		log.Fatalf("Parsing private key error: %v", err)
	}

	pubData, err := os.ReadFile(publicPath)
	if err != nil {
		log.Fatalf("Reading public key error: %v", err)
	}
	rsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(pubData)
	if err != nil {
		log.Fatalf("Reading public key error: %v", err)
	}
}

func GenerateToken(user *User) (string, time.Time, error) {
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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func Authenticate(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	log.Println(tokenString)
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

func GetUserIdByClaims(claims jwt.MapClaims) uint {
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0
	}
	return uint(userIDFloat)
}
