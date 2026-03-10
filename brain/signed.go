package brain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
)

var (
	ErrUnsigned  = errors.New("an attempt was made to verify an unsigned data block")
	ErrBadBase64 = errors.New("base64 decoding failed on input encoded string")
)

// We match ANY backslash and the character immediately following it.
var EscapeFinder = regexp.MustCompile(`\\(.?)`)

func SanitizeJSON(raw []byte) []byte {
	var result []byte
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' {
			if i+1 >= len(raw) {
				continue
			} // Lone trailing backslash

			char := raw[i+1]
			// Case 1: Standard valid escapes
			if strings.ContainsRune(`"/\bfnrt`, rune(char)) {
				result = append(result, '\\', char)
				i++
				continue
			}

			// Case 2: Unicode escapes (\uXXXX)
			if char == 'u' {
				// Check if we have 4 hex digits ahead
				if i+5 < len(raw) && isHex(raw[i+2]) && isHex(raw[i+3]) && isHex(raw[i+4]) && isHex(raw[i+5]) {
					result = append(result, '\\', 'u', raw[i+2], raw[i+3], raw[i+4], raw[i+5])
					i += 5
					continue
				}
				// Truncated or invalid \u: Strip backslash, keep 'u'
				result = append(result, 'u')
				i++
				continue
			}

			// Case 3: Illegal "Greebles" (including \')
			result = append(result, char)
			i++
			continue
		}
		result = append(result, raw[i])
	}
	return result
}

// isHex is the "Forensic" utility needed to validate \u payloads
func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

type BlockType string

const (
	TypeThinking   BlockType = "cot"
	TypeText       BlockType = "text"
	TypePrompt     BlockType = "prompt"
	TypeToolCall   BlockType = "tool_request"
	TypeToolResult BlockType = "tool_response"
	TypeHydration  BlockType = "hydration"
)

type CanSignOrVerify interface {
	Sign(data string) (string, error)
	Verify(data, signature string, publicKey ...string) error
}

type Signed struct {
	Namespace     BlockType `json:"namespace"`
	Data          string    `json:"data"`
	Data64        string    `json:"data_b64"`
	Signature     string    `json:"signature"`
	PrevSignature string    `json:"prev_signature"`
	IsShadow      bool      `json:"is_shadow,omitempty"`
}

func (s *Signed) UnmarshalJSON(b []byte) error {
	type Alias Signed
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	b = SanitizeJSON(b)
	if s.Data64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(s.Data64)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrBadBase64, err)
		}
		s.Data = string(decoded)
	}
	return nil
}

func NewUnsigned(data string, namespace BlockType) Signed {
	return Signed{
		Namespace: namespace,
		Data:      data,
	}
}

func (s *Signed) NextUnsigned(data string, newNamespace ...BlockType) Signed {
	ns := s.Namespace
	if len(newNamespace) > 0 && len(newNamespace[0]) > 0 {
		ns = newNamespace[0]
	}
	return Signed{
		Namespace:     ns,
		Data:          data,
		PrevSignature: s.Signature,
	}
}

func (s *Signed) Sign(sv CanSignOrVerify) error {
	s.Data64 = base64.StdEncoding.EncodeToString([]byte(s.Data))
	if s.Signature != "" { // never sign twice
		return sv.Verify(s.Data64+s.PrevSignature, s.Signature)
	}
	sig, err := sv.Sign(s.Data64 + s.PrevSignature)
	if err != nil {
		return err
	}
	s.Signature = sig
	return nil
}

func (s *Signed) Verify(sv CanSignOrVerify, publicKey ...string) error {
	if s.Signature == "" {
		log.Printf("UNSIGNED: %#v", s)
		return ErrUnsigned
	}
	if s.Data64 == "" {
		s.Data64 = base64.StdEncoding.EncodeToString([]byte(s.Data))
	} else {
		d, err := base64.StdEncoding.DecodeString(s.Data64)
		if err != nil {
			return err
		}
		s.Data = string(d) // Exactly what is in the base64
	}
	b64Data := base64.StdEncoding.EncodeToString([]byte(s.Data))
	return sv.Verify(b64Data+s.PrevSignature, s.Signature, publicKey...)
}
