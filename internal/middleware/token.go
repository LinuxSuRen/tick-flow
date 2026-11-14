package middleware

import (
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenClaims is the JWT payload: sub = user id, tenant = tenant id, role =
// the tenant-scoped role. Expiry is 24h at issuance. UserID carries the
// parsed sub so consumers never re-parse the string.
type tokenClaims struct {
	Tenant int64  `json:"tenant"`
	Role   string `json:"role"`
	UserID int64  `json:"-"`
	jwt.RegisteredClaims
}

func parseBearerToken(header, secret string) (*tokenClaims, error) {
	if header == "" {
		return nil, errMissingToken
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errMissingToken
	}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(parts[1]), &tokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errUnexpectedSigningMethod
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid || claims.Subject == "" {
		return nil, errInvalidClaims
	}
	sub, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || sub <= 0 {
		return nil, errInvalidClaims
	}
	// Tenant 0 with an empty role marks a pre-selection login token: it may
	// only reach /api/me and select-tenant (RequireTenant guards the rest).
	if claims.Tenant < 0 || (claims.Tenant == 0 && claims.Role != "") {
		return nil, errInvalidClaims
	}
	claims.UserID = sub
	return claims, nil
}

// IssueToken signs a JWT valid for 24 hours. It is used by the auth service.
func IssueToken(secret string, userID, tenantID int64, role string, now time.Time) (string, error) {
	claims := tokenClaims{
		Tenant: tenantID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func stackTrace() string {
	return string(debug.Stack())
}

type parseError string

func (e parseError) Error() string { return string(e) }

const (
	errMissingToken          parseError = "missing bearer token"
	errUnexpectedSigningMethod parseError = "unexpected signing method"
	errInvalidClaims         parseError = "invalid token claims"
)
