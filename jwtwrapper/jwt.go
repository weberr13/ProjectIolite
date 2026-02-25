package jwtwrapper

import (
	"crypto/ed25519"

	"github.com/golang-jwt/jwt"
)

type SignVerifier struct {
	privateKey ed25519.PrivateKey // this is a typed []byte 
	publicKey  ed25519.PublicKey // this is a typed []byte 
	method     jwt.SigningMethod
}

func New(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey) *SignVerifier {
	return &SignVerifier{
		privateKey: privateKey,
		publicKey: publicKey,
		method:     jwt.GetSigningMethod("EdDSA"),
	}

}

func (s *SignVerifier) Sign(data string) (string, error) {
	return s.method.Sign(data, s.privateKey)
}

func (s SignVerifier) Verify(data, signature string) error {
	return s.method.Verify(data, signature, s.publicKey)
}