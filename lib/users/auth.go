package users

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type CreateAccessTokenResponse struct {
	AccessToken      string
	RefreshToken     string
	RefreshTokenHash string
}

type AccessTokenClaims struct {
	UserId string `json:"userId"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

func CreateAccessAndRefreshToken(user *User) (CreateAccessTokenResponse, error) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"email":  user.Email,
		"exp":    time.Now().Add(AccessTokenExpiresIn).Unix(),
		"iat":    time.Now().Unix(),
	})

	accessTokenString, err := accessToken.SignedString([]byte("secret"))

	refreshToken, err := GenerateRandomString(32)
	if err != nil {
		return CreateAccessTokenResponse{}, err
	}

	refreshTokenHash, err := HashAndSaltToken(refreshToken)
	if err != nil {
		return CreateAccessTokenResponse{}, err
	}

	return CreateAccessTokenResponse{
		AccessToken:      accessTokenString,
		RefreshToken:     refreshToken,
		RefreshTokenHash: refreshTokenHash,
	}, nil
}

func ParseAccessToken(accessToken string) (AccessTokenClaims, error) {
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil {
		return AccessTokenClaims{}, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok {
		return AccessTokenClaims{}, err
	}

	return AccessTokenClaims{
		UserId: claims["userId"].(string),
		Email:  claims["email"].(string),
		Exp:    int64(claims["exp"].(float64)),
		Iat:    int64(claims["iat"].(float64)),
	}, nil
}

func HashAndSaltToken(token string) (string, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func CompareHashAndPassword(hash string, token string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
}

func GenerateRandomString(size int) (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Encode bytes to base64 string
	str := base64.StdEncoding.EncodeToString(bytes)
	return str, nil
}
