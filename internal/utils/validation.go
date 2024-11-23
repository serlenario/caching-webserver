package utils

import (
	"regexp"
)

func IsValidLogin(login string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)
	return re.MatchString(login)
}

func IsValidPassword(password string) bool {
	hasMinLen := len(password) >= 8
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password)
	return hasMinLen && hasUpper && hasLower && hasNumber && hasSpecial
}
