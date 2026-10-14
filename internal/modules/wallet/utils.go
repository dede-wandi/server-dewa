package wallet

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// FormatRupiah formats an integer amount to Indonesian Rupiah currency string (e.g. 50000 -> "50.000")
func FormatRupiah(n int64) string {
	return formatRupiah(n)
}

func formatRupiah(n int64) string {
	in := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(in)+len(in)/3)
	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// ValidatePIN ensures a PIN is non-empty, exactly 6 characters, and only contains numeric digits (0-9)
func ValidatePIN(pin string) error {
	pin = strings.TrimSpace(pin)
	if len(pin) != 6 {
		return errors.New("PIN transaksi harus tepat 6 digit angka")
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return errors.New("PIN transaksi hanya boleh berisi angka numerik (0-9)")
		}
	}
	return nil
}

// HashPIN securely hashes a 6-digit PIN using bcrypt
func HashPIN(pin string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPIN compares a plaintext PIN against a stored bcrypt hash
func CheckPIN(pin, hash string) bool {
	if hash == "" || pin == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}
