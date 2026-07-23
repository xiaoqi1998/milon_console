package crypto

import "errors"

var (
	ErrInvalidSecretKey = errors.New("invalid secret key")
	ErrInvalidPublicKey = errors.New("invalid public key")
	ErrInvalidSignature = errors.New("invalid signature")
)
