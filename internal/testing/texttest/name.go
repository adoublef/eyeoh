package texttest

import (
	"math/rand"
	"strings"
	"time"
)

// newBucket generates a random, valid S3 bucket name.
// Bucket name length: 3 to 63
func Bucket(n int) string {

	// next generates a random string of a given length
	next := func() string {
		b := make([]byte, 3+seed.Intn(n))
		for i := range b {
			b[i] = charset[seed.Intn(len(charset))]
		}
		return string(b)
	}

	var name string
	for {
		name = next()
		// Ensure the name does not start or end with a hyphen, does not resemble an IP address,
		// and does not contain consecutive periods or invalid characters
		if name[0] != '-' && name[len(name)-1] != '-' && !isIPAddress(name) && !strings.Contains(name, "..") {
			break
		}
	}
	return name
}

// Allowed characters for S3 bucket names (excluding dots)
const charset = "abcdefghijklmnopqrstuvwxyz0123456789-"

// Initialize a random source
var seed *rand.Rand

// init function to seed the random number generator
func init() {
	seed = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// isIPAddress checks if the string is formatted like an IP address
func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) == 4 {
		for _, part := range parts {
			if len(part) == 0 || len(part) > 3 {
				return false
			}
			for _, c := range part {
				if c < '0' || c > '9' {
					return false
				}
			}
		}
		return true
	}
	return false
}
