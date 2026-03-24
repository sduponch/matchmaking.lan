package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"matchmaking.lan/backend/internal/config"
)

type Claims struct {
	SteamID   string `json:"steamid"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWT(steamID, username, avatarURL, role string) (string, error) {
	claims := Claims{
		SteamID:   steamID,
		Username:  username,
		AvatarURL: avatarURL,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.C.JWTExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.C.JWTSecret))
}

func ParseJWT(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(config.C.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
