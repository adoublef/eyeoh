package fs

import (
	"errors"
	"fmt"
)

type Name string

// impl Scanner/Value

func (n Name) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

func (n *Name) UnmarshalText(text []byte) (err error) {
	*n, err = ParseName(string(text))
	if err != nil {
		return err // wrap with json error?
	}
	return nil
}

func (n Name) String() string { return string(n) }

func (n Name) GoString() string { return `Name("` + string(n) + `")` }

func ParseName(s string) (Name, error) {
	if s == "" || len(s) > 255 {
		return "", errors.New("invalid length")
	}
	for _, ch := range s {
		if !isAlphaNumeric(ch) {
			return "", fmt.Errorf("invalid character: '%c'", ch)
		}
	}
	return Name(s), nil
}

func isAlphaNumeric(ch rune) bool {
	// Allowed characters: alphanumeric, hyphen, underscore, and dot
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.'
}
