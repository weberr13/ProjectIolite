package brain

import "errors"

var ErrUnsigned = errors.New("an attempt was made to verify an unsigned data block")

type Signed struct {
	Data          string `json:"data"`
	Signature     string `json:"signature"`
	PrevSignature string `json:"prev_signature,omitempty"`
}

func NewUnsigned(data string) Signed {
	return Signed{
		Data: data,
	}
}

func (s *Signed) NextUnsigned(data string) Signed {
	return Signed{
		Data:          data,
		PrevSignature: s.Signature,
	}
}

func (s *Signed) Sign(sv SignVerifier) error {
	if s.Signature != "" { // never sign twice
		return sv.Verify(s.Data+s.PrevSignature, s.Signature)
	}
	sig, err := sv.Sign(s.Data + s.PrevSignature)
	if err != nil {
		return err
	}
	s.Signature = sig
	return nil
}

func (s *Signed) Verify(sv SignVerifier) error {
	if s.Signature == "" {
		return ErrUnsigned
	}
	return sv.Verify(s.Data+s.PrevSignature, s.Signature)
}
