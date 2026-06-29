package users

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	localPasswordHashPrefix     = "bdk1"
	localPasswordHashIterations = 180000
	localPasswordSaltBytes      = 16
)

func hashLocalPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}

	salt := make([]byte, localPasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	sum := deriveLocalPasswordHash(password, salt, localPasswordHashIterations)
	return fmt.Sprintf("%s$%d$%s$%s",
		localPasswordHashPrefix,
		localPasswordHashIterations,
		hex.EncodeToString(salt),
		hex.EncodeToString(sum),
	), nil
}

func HashLocalPassword(password string) (string, error) {
	return hashLocalPassword(password)
}

func verifyLocalPassword(storedValue, password string) (matched bool, needsUpgrade bool, err error) {
	password = strings.TrimSpace(password)
	if strings.TrimSpace(storedValue) == "" || password == "" {
		return false, false, nil
	}

	parts := strings.Split(storedValue, "$")
	if len(parts) == 4 && parts[0] == localPasswordHashPrefix {
		iterations, err := strconv.Atoi(parts[1])
		if err != nil || iterations <= 0 {
			return false, false, fmt.Errorf("invalid stored password hash format")
		}
		salt, err := hex.DecodeString(parts[2])
		if err != nil {
			return false, false, fmt.Errorf("invalid stored password salt")
		}
		expected, err := hex.DecodeString(parts[3])
		if err != nil {
			return false, false, fmt.Errorf("invalid stored password digest")
		}
		actual := deriveLocalPasswordHash(password, salt, iterations)
		matched := subtle.ConstantTimeCompare(actual, expected) == 1
		return matched, matched && iterations != localPasswordHashIterations, nil
	}

	return subtle.ConstantTimeCompare([]byte(storedValue), []byte(password)) == 1, true, nil
}

func deriveLocalPasswordHash(password string, salt []byte, iterations int) []byte {
	block := make([]byte, 0, len(salt)+len(password)+sha256.Size)
	block = append(block, salt...)
	block = append(block, password...)

	sum := sha256.Sum256(block)
	digest := sum[:]
	for i := 1; i < iterations; i++ {
		next := make([]byte, 0, len(salt)+len(password)+len(digest))
		next = append(next, salt...)
		next = append(next, password...)
		next = append(next, digest...)
		sum = sha256.Sum256(next)
		digest = sum[:]
	}
	out := make([]byte, len(digest))
	copy(out, digest)
	return out
}
