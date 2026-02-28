package brain

import (
	"encoding/base64"
	"errors"
)

var ErrUnsigned = errors.New("an attempt was made to verify an unsigned data block")

type CanSignOrVerify interface {
	Sign(data string) (string, error)
	Verify(data, signature string) error
}

type Signed struct {
	Namespace     string `json:"namespace"`
	Data          string `json:"data"`
	Signature     string `json:"signature"`
	PrevSignature string `json:"prev_signature"`
}

func NewUnsigned(data, namespace string) Signed {
	return Signed{
		Namespace: namespace,
		Data:      data,
	}
}

func (s *Signed) NextUnsigned(data string) Signed {
	return Signed{
		Namespace:     s.Namespace,
		Data:          data,
		PrevSignature: s.Signature,
	}
}

func (s *Signed) Sign(sv CanSignOrVerify) error {
	b64Data := base64.StdEncoding.EncodeToString([]byte(s.Data))
	if s.Signature != "" { // never sign twice
		return sv.Verify(b64Data+s.PrevSignature, s.Signature)
	}
	sig, err := sv.Sign(b64Data + s.PrevSignature)
	if err != nil {
		return err
	}
	s.Signature = sig
	return nil
}

func (s *Signed) Verify(sv CanSignOrVerify) error {
	if s.Signature == "" {
		return ErrUnsigned
	}
	b64Data := base64.StdEncoding.EncodeToString([]byte(s.Data))
	return sv.Verify(b64Data+s.PrevSignature, s.Signature)
}
