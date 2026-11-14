// Package service holds the domain logic: state machines, validation
// orchestration, event publication after commit. Handlers stay thin.
package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// inviteTokenLifetime bounds invitation links (issue #6: single use,
// expiring).
const inviteTokenLifetime = 72 * time.Hour

// ErrInvalidInvite reports a malformed, expired or already used invitation.
var ErrInvalidInvite = errors.New("invitation is invalid, expired or already used")

// inviteToken is a self-contained HMAC-signed invitation: tenant, email,
// role and expiry. Single-use semantics come from the user status flip:
// once the account is active the token no longer matches an invited user.
type inviteToken struct {
	TenantID int64
	Email    string
	Role     string
	Expires  time.Time
}

func (t inviteToken) payload() string {
	return strings.Join([]string{
		"invite", strconv.FormatInt(t.TenantID, 10), strings.ToLower(t.Email), t.Role,
		strconv.FormatInt(t.Expires.Unix(), 10),
	}, "|")
}

// signInvite produces the opaque invitation string handed to the invitee.
func signInvite(secret string, t inviteToken) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(t.payload())); err != nil {
		return "", err
	}
	sig := mac.Sum(nil)
	raw := t.payload() + "|" + hex.EncodeToString(sig)
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// parseInvite verifies signature and expiry.
func parseInvite(secret, token string) (*inviteToken, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return nil, ErrInvalidInvite
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 6 || parts[0] != "invite" {
		return nil, ErrInvalidInvite
	}
	payload := strings.Join(parts[:5], "|")
	want, err := hex.DecodeString(parts[5])
	if err != nil {
		return nil, ErrInvalidInvite
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	if !hmac.Equal(mac.Sum(nil), want) {
		return nil, ErrInvalidInvite
	}
	tenantID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, ErrInvalidInvite
	}
	expUnix, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return nil, ErrInvalidInvite
	}
	t := &inviteToken{
		TenantID: tenantID,
		Email:    parts[2],
		Role:     parts[3],
		Expires:  time.Unix(expUnix, 0),
	}
	if time.Now().After(t.Expires) {
		return nil, ErrInvalidInvite
	}
	return t, nil
}

// randomPassword produces an unguessable placeholder for invited accounts.
// Nobody logs in with it; the invitee sets a real password via the link.
func randomPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random password: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
