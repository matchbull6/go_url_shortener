package shortener

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const DefaultCodeLength = 7
const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) (string, error) {
	var sb strings.Builder
	for range length {
		bigInt, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		sb.WriteByte(chars[bigInt.Int64()])
	}
	return sb.String(), nil
}
