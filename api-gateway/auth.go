// auth.go (в проекте api-gateway)
package main

import (
	"crypto/rsa"
	"fmt"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

var rsaPublicKey *rsa.PublicKey

func LoadRSAKeys(publicPath string) {

	pubData, err := os.ReadFile(publicPath)
	if err != nil {
		log.Fatalf("Reading public key error: %v", err)
	}
	rsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(pubData)
	if err != nil {
		log.Fatalf("Reading public key error: %v", err)
	}
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
