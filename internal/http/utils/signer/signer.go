package signer

import (
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type Signer struct {
	secret string
}

func New(secret string) *Signer {
	return &Signer{
		secret: secret,
	}
}

func (s *Signer) Sign(c interface{}, t time.Time) (string, error) {
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(s.secret)},
		(&jose.SignerOptions{}).WithType("JWT"),
	)

	if err != nil {
		return "", err
	}

	claims := jwt.Claims{
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Expiry:   jwt.NewNumericDate(t),
	}

	return jwt.Signed(sig).Claims(claims).Claims(c).Serialize()
}

func (s *Signer) Verify(t string, c interface{}) error {
	tok, err := jwt.ParseSigned(t, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return err
	}
	var std jwt.Claims
	if err := tok.Claims([]byte(s.secret), &std, c); err != nil {
		return err
	}
	return std.Validate(jwt.Expected{Time: time.Now()})
}
