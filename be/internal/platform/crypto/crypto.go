package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"govite/internal/policy"
)

// Argon2id parameters match node-argon2 defaults, so hashes move both ways.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

var b64 = base64.RawStdEncoding

// HashPassword returns a PHC string: $argon2id$v=19$m=65536,t=3,p=4$salt$hash.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks plain against a PHC argon2id hash in constant time.
func VerifyPassword(phc, plain string) bool {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyHash keeps login timing equal for unknown users. Never a real credential.
const DummyHash = "$argon2id$v=19$m=65536,t=3,p=4$7YKz1pWben/1QhI2b4OqEw$3uNsos3EmzAM4vYB90VcRMWZF26F21DJAm1qqM7hlrQ"

// Keys groups the server secrets.
type Keys struct {
	jwt    []byte
	pepper []byte
	aead   cipher.AEAD
}

func NewKeys(jwtSecret, pepper string) Keys {
	sum := sha256.Sum256([]byte(pepper))
	block, _ := aes.NewCipher(sum[:])
	aead, _ := cipher.NewGCM(block)
	return Keys{jwt: []byte(jwtSecret), pepper: []byte(pepper), aead: aead}
}

// HMAC keys token hashes with the pepper, so a DB dump alone can't verify guesses.
func (k Keys) HMAC(v string) string {
	m := hmac.New(sha256.New, k.pepper)
	m.Write([]byte(v))
	return hex.EncodeToString(m.Sum(nil))
}

// Encrypt: base64url(iv | tag | ciphertext), same layout as the Nest version.
func (k Keys) Encrypt(plain string) string {
	iv := make([]byte, 12)
	_, _ = rand.Read(iv)
	sealed := k.aead.Seal(nil, iv, []byte(plain), nil)
	ct, tag := sealed[:len(sealed)-16], sealed[len(sealed)-16:]
	return base64.RawURLEncoding.EncodeToString(append(append(iv, tag...), ct...))
}

func (k Keys) Decrypt(enc string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || len(raw) < 28 {
		return "", errors.New("bad ciphertext")
	}
	iv, tag, ct := raw[:12], raw[12:28], raw[28:]
	out, err := k.aead.Open(nil, iv, append(append([]byte{}, ct...), tag...), nil)
	return string(out), err
}

type Claims struct {
	RoleID string `json:"roleId"`
	SID    string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

func (k Keys) SignAccess(userID, roleID, sid string) (string, int, error) {
	ttl := policy.Security.AccessTokenTTL
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RoleID: roleID, SID: sid,
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl))},
	})
	s, err := tok.SignedString(k.jwt)
	return s, int(ttl.Seconds()), err
}

func (k Keys) ParseAccess(token string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return k.jwt, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || c.Subject == "" {
		return nil, errors.New("invalid token")
	}
	return &c, nil
}

// RandomToken returns n random bytes as base64url.
func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// RandomDigits returns a zero-padded random decimal of length n.
func RandomDigits(n int) string {
	max := big.NewInt(1)
	for range n {
		max.Mul(max, big.NewInt(10))
	}
	v, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%0*d", n, v)
}

func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
