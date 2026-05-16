package validator

import "regexp"

func ValidateEmail(email string) bool {
	regexEmail := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	matched, err := regexp.MatchString(regexEmail, email)
	return err == nil && matched
}
