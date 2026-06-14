package testutil

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// LaravelBcryptHash returns a bcrypt hash compatible with Laravel's $2y$ format.
func LaravelBcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return strings.Replace(string(hash), "$2a$", "$2y$", 1), nil
}

// VerifyLaravelBcrypt checks a password against a Laravel bcrypt hash.
func VerifyLaravelBcrypt(password, hash string) bool {
	normalized := strings.Replace(hash, "$2y$", "$2a$", 1)
	return bcrypt.CompareHashAndPassword([]byte(normalized), []byte(password)) == nil
}
