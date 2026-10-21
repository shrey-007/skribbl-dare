package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"skribbl-dare-backend/internal/models"
)

type JWTClaims struct {
	UserID    string `json:"user_id"`
	DiscordID string `json:"discord_id"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(user *models.User, secret string) (string, error) {
	claims := JWTClaims{
		UserID:    user.ID.Hex(),
		DiscordID: user.DiscordID,
		Username:  user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)), // 30 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "skribbl-dare-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenStr string, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
