package jwtwrapper

import (
	"crypto/ed25519"
	_ "embed" // Required for the embed directive
	"encoding/base64"

	"github.com/golang-jwt/jwt"
	"golang.org/x/text/unicode/norm"
)

//go:embed edDSA.py
var edDSA_py string

const signMethod = "EdDSA"

var DecodePy = map[string]string{
	"EdDSA": edDSA_py,
}

type SignVerifier struct {
	privateKey ed25519.PrivateKey // this is a typed []byte
	publicKey  ed25519.PublicKey  // this is a typed []byte
	method     jwt.SigningMethod
}

func New(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey) *SignVerifier {
	sv := &SignVerifier{
		privateKey: privateKey,
		publicKey:  publicKey,
		method:     jwt.GetSigningMethod(signMethod),
	}
	return sv
}

func (s *SignVerifier) VerifyPy() string {
	return DecodePy[signMethod]
}

func (s *SignVerifier) Sign(data string) (string, error) {
	canonicalData := norm.NFC.String(data)
	return s.method.Sign(canonicalData, s.privateKey)
}

func (s SignVerifier) Verify(data, signature string, publicKey ...string) error {
	pk := s.publicKey
	var err error
	if len(publicKey) > 0 {
		pk, err = base64.StdEncoding.DecodeString(publicKey[0])
		if err != nil {
			return err
		}
	}
	return s.method.Verify(data, signature, pk)
}

func (s *SignVerifier) ExportPublicKey() string {
	return base64.StdEncoding.EncodeToString(s.publicKey)
}

func (s *SignVerifier) Alg() string {
	return signMethod
}
