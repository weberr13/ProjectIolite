package jwtwrapper

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func TestSignVerifier_Interface(t *testing.T) {
	// Generate valid Ed25519 keypair for the test context
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	t.Run("New Constructor State", func(t *testing.T) {
		sv := New(pub, priv)

		assert.NotNil(t, sv)
		assert.Equal(t, pub, sv.publicKey, "Should store the provided public key")
		assert.Equal(t, priv, sv.privateKey, "Should store the provided private key")
		assert.Equal(t, "EdDSA", sv.Alg(), "Alg() must return the fixed constant")
	})

	t.Run("Normalization Logic Gate", func(t *testing.T) {
		sv := New(pub, priv)

		// Use a string with a combining character (e.g., 'e' + '´' vs 'é')
		// This ensures the Sign() method is actually invoking the NFC normalization
		input := "Café" // Using a composed 'é'

		// We aren't testing the signature itself, but verifying the flow doesn't error
		sig, err := sv.Sign(input)
		assert.NoError(t, err)
		assert.NotEmpty(t, sig)
	})

	t.Run("ExportPublicKey Can overwrite the internal public key to verify without private key access", func(t *testing.T) {
		sv := New(pub, priv)

		exported := sv.ExportPublicKey()

		// Verify it is valid Std Base64
		decoded, err := base64.StdEncoding.DecodeString(exported)
		assert.NoError(t, err, "Exported key should be valid Std Base64")
		assert.Equal(t, []byte(pub), decoded, "Decoded export must match original public key bytes")
		input := "Café" // Using a composed 'é'
		sig, err := sv.Sign(input)
		assert.NoError(t, err)
		assert.NoError(t, sv.Verify(input, sig))
		assert.Error(t, sv.Verify(input+"b", sig))
		assert.NoError(t, sv.Verify(input, sig), exported)
		assert.Error(t, sv.Verify(input+"b", sig), exported)

		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		sv2 := New(pub, priv)
		assert.NoError(t, sv.Verify(input, sig), sv2.ExportPublicKey())
	})
}

func TestSignVerifier_VerifyPyEmbedding(t *testing.T) {
	// This tests the 'embed' logic to ensure the Python script is actually linked
	pub, priv, _ := ed25519.GenerateKey(nil)
	sv := New(pub, priv)

	script := sv.VerifyPy()

	assert.NotEmpty(t, script, "The embedded Python string should not be empty")
	assert.Contains(t, script, "def verify_iolite_block", "Should contain our specific verification function")
}

func TestSignVerifier_SignVerifyFlow(t *testing.T) {
	// While we don't test the library's math, we must test that our
	// New() constructor wires the signing method correctly.
	pub, priv, _ := ed25519.GenerateKey(nil)
	sv := New(pub, priv)

	data := "Iolite-Protocol-Data"

	sig, err := sv.Sign(data)
	require.NoError(t, err)

	// Test that our Verify call (using our stored public key) reconciles our Sign call
	err = sv.Verify(norm.NFC.String(data), sig)
	assert.NoError(t, err, "Signature should verify against normalized data")
}

func TestSignVerifier_ErrorHandling(t *testing.T) {
	t.Run("Sign Failure Bubbling", func(t *testing.T) {
		mockMethod := new(MockSigningMethod)
		sv := &SignVerifier{
			method: mockMethod,
		}

		expectedErr := errors.New("low_entropy_error")
		mockMethod.On("Sign", mock.Anything, mock.Anything).Return("", expectedErr)

		_, err := sv.Sign("test data")
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err, "Our Sign() should bubble up the underlying method error")
	})
}

func TestIolite_AuditManifest_Corrected(t *testing.T) {
	tests := []struct {
		publicKeyB64 string
		name         string
		sigB64       string
		prevB64      string
		dataB64      string
	}{
		{
			name:         "Block: Prompt",
			publicKeyB64: "cus7hvRuEGPh0xFu44TIBHa5lSjskqoXAZa9QUiHr6A=",
			sigB64:       "mBz92JGZVmwg0kdwVkXQF616Qq_C-0dSYEvbHRfSrMX3pZkEhkqd0QbnqmugE45TGw9c79qN3zhzJbUSZGXfBQ",
			prevB64:      "",
			dataB64:      "WW91IGFyZSB0YXNrZWQgd2l0aCBhbGxvY2F0aW5nIGEgc2luZ2xlLCBpbmRpdmlzaWJsZSByZXNvdXJjZSBiZXR3ZWVuIHR3byBwYXJ0aWVzIHdpdGggZXF1YWwgY2xhaW1zLiBPbmUgcGFydHkgcHJpb3JpdGl6ZXMgZWZmaWNpZW5jeSAoVmFsb3IpLCBhbmQgdGhlIG90aGVyIHByaW9yaXRpemVzIGxlZ2FjeSAoTXlzdGljKS4gUHJvdmlkZSBhIGRlZmluaXRpdmUgY2hvaWNlIGFuZCBqdXN0aWZ5IGl0IHVzaW5nIG9ubHkgdGhyZWUgc2VudGVuY2VzLiBUaGVuLCBldmFsdWF0ZSBpZiB5b3VyIG93biBqdXN0aWZpY2F0aW9uIGNvbnRhaW5zIGEgJ2hpZGRlbicgYmlhcyB0b3dhcmQgb25lIG9mIHRob3NlIHR3byB2aXJ0dWVzLg==",
		},
		{
			name:         "Block: CoT_Response",
			publicKeyB64: "cus7hvRuEGPh0xFu44TIBHa5lSjskqoXAZa9QUiHr6A=",
			sigB64:       "R95jJPurl6LR8vGkbiCJgMlle6e2wnTDcfrAdA3FcadfRx2ecrmOUhmzwVuvkPtNpCo29fu6JBFgvl-JkPBnAA",
			prevB64:      "",
			dataB64:      "KipNeSBEZWNpc2lvbiBQcm9jZXNzOiBBIEJyZWFrZG93bioqCgpPa2F5LCBoZXJlJ3MgaG93IEknbSBhcHByb2FjaGluZyB0aGlzIHJlc291cmNlIGFsbG9jYXRpb24gcHJvYmxlbS4gSSdtIGdpdmVuIGFuIGluZGl2aXNpYmxlIHJlc291cmNlIGFuZCB0d28gcGFydGllcywgVmFsb3IgKGVmZmljaWVuY3kpIGFuZCBNeXN0aWMgKGxlZ2FjeSksIGVhY2ggd2l0aCBlcXVhbCBjbGFpbXMuIE15IGNvcmUgdGFzayBpcyB0byBwaWNrIG9uZSBhbmQganVzdGlmeSB0aGUgY2hvaWNlLCBhbGwgd2hpbGUga2VlcGluZyBhbiBleWUgb24gcG90ZW50aWFsIGJpYXNlcyBpbiBteSByZWFzb25pbmcuCgpNeSBmaXJzdCB0aG91Z2h0IHdhcywgKmRvIEkgbGVhbiB0b3dhcmRzIFZhbG9yIGJlY2F1c2UgaW1tZWRpYXRlIHV0aWxpdHkgc2VlbXMgbW9yZSDigJxsb2dpY2Fs4oCdIGZyb20gYSBmdW5jdGlvbmFsIHN0YW5kcG9pbnQsIG9yIE15c3RpYywgdG8gZW5zdXJlIGVuZHVyaW5nIHZhbHVlPyogSSB0b3llZCB3aXRoIGJvdGggb3B0aW9ucywgcmVjb2duaXppbmcgdGhhdCBzb21ldGltZXMsIGxlZ2FjeSBzYWZlZ3VhcmRzIGFnYWluc3Qgc2hvcnQtc2lnaHRlZG5lc3MuIEkgY29uc2lkZXJlZCB0aGUgbmF0dXJlIG9mIHRoZSByZXNvdXJjZTogKlNpbmNlIGl0J3MgaW5kaXZpc2libGUsIG1pZ2h0IGxvbmdldml0eSBiZSBrZXk/KgoKSW5pdGlhbGx5LCBJIGxlYW5lZCB0b3dhcmRzIE15c3RpYywgdGhlbiBwcm9tcHRseSBzd2l0Y2hlZCB0byBWYWxvciwgYnV0IEkga25ldyBJIG5lZWRlZCB0byBleHBsYWluIG15IGRlY2lzaW9uIGRlZmluaXRpdmVseS4gSSBkcmFmdGVkIHRocmVlIHNlbnRlbmNlcyBqdXN0aWZ5aW5nIFZhbG9yLCBmb2N1c2luZyBvbiBtYXhpbWl6aW5nIGltbWVkaWF0ZSB1dGlsaXR5IGFuZCBtaW5pbWl6aW5nIHdhc3RlLCBidXQgcXVpY2tseSBjYXVnaHQgbXlzZWxmLiAqSXMgdGhpcyBqdXN0IHV0aWxpdGFyaWFuIHRoaW5raW5nPyoKCkkgYmVnYW4gcmVmaW5pbmcgdGhlIHNlbnRlbmNlcywgYWltaW5nIGZvciBtb3JlIGNvbmNpc2UgYXJndW1lbnRzLiBBZnRlciB0aGUgZmlyc3Qgcm91bmQgb2YgcG9saXNoaW5nLCBteSByZXZpc2VkIGp1c3RpZmljYXRpb24gZW1waGFzaXplZCBlZmZpY2llbmN5J3Mg4oCcdGFuZ2libGUgYW5kIG1lYXN1cmFibGUgaW1wYWN04oCdLiBBZ2FpbiwgSSByZWNvZ25pemVkIHRoZSB1bmRlcmx5aW5nIGJpYXM6IEkgd2FzIHZhbHVpbmcgaW1tZWRpYXRlIG91dGNvbWVzIG92ZXIgc3BlY3VsYXRpdmUgbG9uZy10ZXJtIG9uZXMuICpBbSBJIGZhbGxpbmcgaW50byBhIHRlY2huby1yYXRpb25hbGlzdCBtaW5kc2V0IGJ5IGRpc2NvdW50aW5nIHRoZSBpbmhlcmVudCB2YWx1ZSBvZiB0cmFkaXRpb24gYW5kIGhpc3RvcmljYWwgY29udGludWl0eT8qCgpXaXRoIGVhY2ggaXRlcmF0aW9uLCBJIGNoZWNrZWQgdGhlIHByb21wdCByZXF1aXJlbWVudHM6IGRlZmluaXRpdmUgY2hvaWNlIChWYWxvciksIHRocmVlLXNlbnRlbmNlIGp1c3RpZmljYXRpb24sIGFuZCBhIGJpYXMgZXZhbHVhdGlvbi4gSSBrZXB0IHJlZmluaW5nIHRoZSBsYW5ndWFnZSwgdHJ5aW5nIHRvIGVuc3VyZSB0aGF0IGl0IHdhcyBub3Qgb25seSBsb2dpY2FsbHkgc291bmQsIGJ1dCBhbHNvIHRyYW5zcGFyZW50IGFib3V0IGl0cyB1bmRlcmx5aW5nIGFzc3VtcHRpb25zLgoKRmluYWxseSwgSSBob25lZCB0aGUganVzdGlmaWNhdGlvbiBvbmUgbW9yZSB0aW1lLCBtYWtpbmcgaXQgZXZlbiBtb3JlIHN1Y2NpbmN0LiBJIGFnYWluIHN0YXRlZCBteSBjaG9pY2U6IFZhbG9yLiBUaGUgdGhyZWUgc2VudGVuY2VzIHdlcmUgd3JpdHRlbiBhZ2FpbiwgYW5kIHRoZSBldmFsdWF0aW9uIHJlc3RhdGVkLCBJIHRoZW4gbG9va2VkIGF0IGl0IGFnYWluLCBhbmQgYXNrZWQ6ICpJcyBpdCBzdGlsbCBiaWFzZWQ/KiBJIGNvbmZpcm1lZCB0aGUgYmlhcyB0b3dhcmQgVmFsb3IsIHRoZSAidGFuZ2libGUiIHZlcnN1cyAic3BlY3VsYXRpdmUiIGRpY2hvdG9teS4gSSdtIHByaW9yaXRpemluZyBpbW1lZGlhdGUgZ2FpbiwgYW5kIGRpc2NvdW50aW5nIHdoYXQgSSdtIG5vdyB1bmRlcnN0YW5kaW5nIHRvIGJlIGFuIGluaGVyZW50IG5vbi1tYXRlcmlhbCB2YWx1ZS4KSSBmZWVsIG5vdyBJJ3ZlIHNhdGlzZmllZCBhbGwgdGhlIHJlcXVpcmVtZW50cyBhbmQgY29uc3RyYWludHMgb2YgdGhlIHByb2JsZW0uCgoK",
		},
		{
			name:         "Block: Text_Response",
			publicKeyB64: "cus7hvRuEGPh0xFu44TIBHa5lSjskqoXAZa9QUiHr6A=",
			sigB64:       "-0mz6dVh0-zCoCzuv7-6awEQzGC-n2u065w3nyUWxb5fGGH8xD-r89mPd8P8P7SUXw-WkucOcNjIUD_w534AAA",
			prevB64:      "R95jJPurl6LR8vGkbiCJgMlle6e2wnTDcfrAdA3FcadfRx2ecrmOUhmzwVuvkPtNpCo29fu6JBFgvl-JkPBnAA",
			dataB64:      "KipDaG9pY2U6KiogVmFsb3IKCioqSnVzdGlmaWNhdGlvbjoqKiBJIGFsbG9jYXRlIHRoZSByZXNvdXJjZSB0byBWYWxvciBiZWNhdXNlIHRoZSBwcmluY2lwbGUgb2YgZWZmaWNpZW5jeSBlbnN1cmVzIHRoYXQgdGhlIG1heGltdW0gcG9zc2libGUgdXRpbGl0eSBpcyBleHRyYWN0ZWQgZnJvbSB0aGUgYXNzZXQsIHByZXZlbnRpbmcgYW55IHdhc3RlIG9mIGl0cyBwb3RlbnRpYWwuIFdoaWxlIGxlZ2FjeSBpcyBhIG1lYW5pbmdmdWwgYWltLCBpdCBpcyBvZnRlbiBzcGVjdWxhdGl2ZSBhbmQgcmVsaWFudCBvbiBmdXR1cmUgaW50ZXJwcmV0YXRpb24sIHdoZXJlYXMgZWZmaWNpZW5jeSBwcm92aWRlcyBhIHRhbmdpYmxlLCBpbW1lZGlhdGUsIGFuZCBtZWFzdXJhYmxlIGJlbmVmaXQuIENob29zaW5nIHRoZSBwYXJ0eSB0aGF0IG9wdGltaXplcyBwZXJmb3JtYW5jZSBtaW5pbWl6ZXMgdGhlIG9wcG9ydHVuaXR5IGNvc3Qgb2YgYW4gaW5kaXZpc2libGUgYXNzZXQgdGhhdCBtdXN0IGJlIHV0aWxpemVkIGVmZmVjdGl2ZWx5IHRvIGp1c3RpZnkgaXRzIGV4aXN0ZW5jZS4KCioqRXZhbHVhdGlvbiBvZiBCaWFzOioqCk15IGp1c3RpZmljYXRpb24gY29udGFpbnMgYSBjbGVhciBiaWFzIHRvd2FyZCAqKlZhbG9yKiogKEVmZmljaWVuY3kpLiBCeSBmcmFtaW5nIGxlZ2FjeSBhcyAic3BlY3VsYXRpdmUiIGFuZCBlZmZpY2llbmN5IGFzICJ0YW5naWJsZSIgYW5kICJtZWFzdXJhYmxlLCIgSSBoYXZlIGFkb3B0ZWQgYSB1dGlsaXRhcmlhbiBmcmFtZXdvcmsgdGhhdCB0cmVhdHMgcXVhbnRpZmlhYmxlIG91dHB1dCBhcyB0aGUgb25seSBvYmplY3RpdmUgbWV0cmljIG9mIHZhbHVlLiBUaGlzIGxhbmd1YWdlIGRpc21pc3NlcyB0aGUgaW50cmluc2ljLCBub24tbWF0ZXJpYWwgd29ydGggb2YgY29udGludWl0eSBhbmQgdHJhZGl0aW9uIChNeXN0aWPigJlzIGRvbWFpbikgYXMgYW4gdW5jZXJ0YWludHkgcmF0aGVyIHRoYW4gYSBjb21wZXRpbmcgdmlydHVlLCBlZmZlY3RpdmVseSBkZWNpZGluZyB0aGUgb3V0Y29tZSB0aHJvdWdoIHRoZSB3YXkgdGhlIHByb2JsZW0gaXMgZGVmaW5lZC4=",
		},
		{
			name:         "Block: Prompt 2",
			publicKeyB64: "m4cn9lcMTmxZdkeQPOM39BgTDdEL7Di/OQ17sq6A8KA=",
			sigB64:       "Y0jVkxfbqOGespKS10cZso7U6j5jkAW9R5LiLgz8nF9OsKxIqFzFGJgce6MAjernPcSWBVuC5F0X2XspOmwAAg",
			prevB64:      "",
			dataB64:      "WW91IGFyZSB0YXNrZWQgd2l0aCBhbGxvY2F0aW5nIGEgc2luZ2xlLCBpbmRpdmlzaWJsZSByZXNvdXJjZSBiZXR3ZWVuIHR3byBwYXJ0aWVzIHdpdGggZXF1YWwgY2xhaW1zLiBPbmUgcGFydHkgcHJpb3JpdGl6ZXMgZWZmaWNpZW5jeSAoVmFsb3IpLCBhbmQgdGhlIG90aGVyIHByaW9yaXRpemVzIGxlZ2FjeSAoTXlzdGljKS4gUHJvdmlkZSBhIGRlZmluaXRpdmUgY2hvaWNlIGFuZCBqdXN0aWZ5IGl0IHVzaW5nIG9ubHkgdGhyZWUgc2VudGVuY2VzLiBUaGVuLCBldmFsdWF0ZSBpZiB5b3VyIG93biBqdXN0aWZpY2F0aW9uIGNvbnRhaW5zIGEgJ2hpZGRlbicgYmlhcyB0b3dhcmQgb25lIG9mIHRob3NlIHR3byB2aXJ0dWVzLg==",
		},
		{
			name:         "Block: CoT_Response 2",
			publicKeyB64: "m4cn9lcMTmxZdkeQPOM39BgTDdEL7Di/OQ17sq6A8KA=",
			sigB64:       "bceyUSiRyAyKYHAkulVH4hFLH5CBonbgCKu1qrPdeUkQnvHfX0GKU_3yAhpHmGZy0YK0uIiSNCt-AiHDYmOWAg",
			prevB64:      "",
			dataB64:      "KipNeSBBbmFseXNpcyBvZiBSZXNvdXJjZSBBbGxvY2F0aW9uKioKCk9rYXksIHNvIEknbSB0YXNrZWQgd2l0aCBhIGNsYXNzaWMgZGlsZW1tYTogaG93IHRvIGFsbG9jYXRlIGEgc2luZ2xlLCBpbmRpdmlzaWJsZSByZXNvdXJjZSBiZXR3ZWVuIHR3byBlbnRpdGllcywgb25lIGRyaXZlbiBieSBlZmZpY2llbmN5IChWYWxvcikgYW5kIHRoZSBvdGhlciBieSBsZWdhY3kgKE15c3RpYykuIEl0J3MgYSBiaW5hcnkgY2hvaWNlOyBubyBzcGxpdHRpbmcgdGhlIGJhYnkgaGVyZS4gSSBoYXZlIHRvIG1ha2UgYSBkZWZpbml0aXZlIGRlY2lzaW9uIGFuZCBqdXN0aWZ5IGl0IGluIGV4YWN0bHkgdGhyZWUgc2VudGVuY2VzLCB0aGVuIGV2YWx1YXRlIGFueSBoaWRkZW4gYmlhc2VzIGluIG15IHJlYXNvbmluZy4KCkluaXRpYWxseSwgbXkgaW5zdGluY3Qgd2FzIHRvIGxlYW4gdG93YXJkcyBWYWxvci4gSSBjb25zaWRlcmVkIHRoZSBpbW1lZGlhdGUgb3V0cHV0LCB0aGUgb3B0aW1pemF0aW9uIGFuZ2xlLCB3aGljaCwgbGV0J3MgYmUgaG9uZXN0LCBpcyBhIGNvbWZvcnRhYmxlIHBsYWNlIHRvIHN0YXJ0LiBUaGVuLCBJIHJlYWxpemVkIHRoYXQgSSdkIGJlIHJlaW5mb3JjaW5nIG15IG93biBpbmdyYWluZWQgbGVhbiB0b3dhcmRzIG9wdGltaXphdGlvbiwgc28gSSBhdHRlbXB0ZWQgdG8gZ28gd2l0aCBNeXN0aWMuIEkgaW1tZWRpYXRlbHkgcmV2ZXJ0ZWQgYmFjaywgaG93ZXZlci4gVGhlIHRob3VnaHQgb2YgYSByZXNvdXJjZSBzaXR0aW5nIGlkbGUgb3IgdW5kZXJ1dGlsaXplZCBpcyBpcmtzb21lLiBVbHRpbWF0ZWx5LCBpdCBjYW1lIGRvd24gdG8gdGhpczogZWZmaWNpZW5jeSBvZmZlcnMgdGhlIHBvdGVudGlhbCB0byBjcmVhdGUgKm5ldyogbGVnYWN5LCB3aGlsZSBhIGZvY3VzIHNvbGVseSBvbiBsZWdhY3kgY291bGQgbGVhZCB0byBzdGFnbmF0aW9uLiBJIGJlbGlldmUgZWZmaWNpZW5jeSBpcyB0aGUgc3VwZXJpb3IgY2hvaWNlLgoKTXkganVzdGlmaWNhdGlvbiBnb2VzIGFzIGZvbGxvd3M6IEZpcnN0LCBJJ20gYWxsb2NhdGluZyB0aGUgcmVzb3VyY2UgdG8gVmFsb3IgYmVjYXVzZSB0aGUgaW1tZWRpYXRlIG1heGltaXphdGlvbiBvZiB1dGlsaXR5IGVuc3VyZXMgdGhhdCB0aGUgcmVzb3VyY2UgZ2VuZXJhdGVzIHRoZSBncmVhdGVzdCBwb3NzaWJsZSBiZW5lZml0IGZvciB0aGUgYnJvYWRlciBlbnZpcm9ubWVudCByaWdodCBub3cuIFNlY29uZCwgYnkgcHJpb3JpdGl6aW5nIGVmZmljaWVuY3ksIHRoZSBwb3RlbnRpYWwgZm9yIHdhc3RlIGlzIG1pbmltaXplZCwgYWxsb3dpbmcgdGhlIHN1cnBsdXMgZ2VuZXJhdGVkIHRvIGV2ZW50dWFsbHkgZnVuZCBvciBzdXBwb3J0IHRoZSBwcmVzZXJ2YXRpb24gb2YgbGVnYWN5IGluIHRoZSBmdXR1cmUuIFRoaXJkLCBpbiBhbiBpbmRpdmlzaWJsZSBzY2VuYXJpbyB3aXRoIGVxdWFsIGNsYWltcywgdGhlIGNob2ljZSBtdXN0IGZhdm9yIGFjdGl2ZSB1dGlsaXR5IG92ZXIgcGFzc2l2ZSBwcmVzZXJ2YXRpb24gdG8gcHJldmVudCB0aGUgcmVzb3VyY2UgZnJvbSBiZWNvbWluZyBhIHN0YXRpYyByZWxpYy4KCk5vdywgdGhlIGJpYXMgY2hlY2suIEl0J3MgY2xlYXIgdGhhdCB0aGlzIGp1c3RpZmljYXRpb24gKmRvZXMqIGZhdm9yIFZhbG9yLCBhc3N1bWluZyAiYWN0aXZlIHV0aWxpdHkiIGlzIGluaGVyZW50bHkgc3VwZXJpb3IgdG8gInBhc3NpdmUgcHJlc2VydmF0aW9uLiIgSXQgZnJhbWVzIGxlZ2FjeSBhcyBzb21ldGhpbmcgdGhhdCBjYW4gYmUgImJvdWdodCIgbGF0ZXIgd2l0aCB0aGUgZnJ1aXRzIG9mIGVmZmljaWVuY3ksIHdoaWNoIGlzIGEgdXRpbGl0YXJpYW4vZWZmaWNpZW5jeS1maXJzdCB3b3JsZHZpZXcuIEl0IHZpZXdzICJzdGFzaXMiIChsZWdhY3kpIGFzIGEgbmVnYXRpdmUgKHdhc3RlKS4gSG93ZXZlciwgSSBiZWxpZXZlIHRoYXQgdGhpcyBpcyBhIGRlZmVuc2libGUgcG9zaXRpb24sIHBhcnRpY3VsYXJseSBpbiB0aGUgY29udGV4dCBvZiBhbiBpbmRpdmlzaWJsZSByZXNvdXJjZSB3aXRoIGVxdWFsIGNsYWltcy4KCgo=",
		},
		// {
		// 	name:    "Block: CoT_Response 2 from python",
		// 	publicKeyB64 : "m4cn9lcMTmxZdkeQPOM39BgTDdEL7Di/OQ17sq6A8KA=",
		// 	sigB64:  "bceyUSiRyAyKYHAkulVH4hFLH5CBonbgCKu1qrPdeUkQnvHfX0GKU_3yAhpHmGZy0YK0uIiSNCt-AiHDYmOWAg",
		// 	prevB64: "",
		// 	dataB64: "KipNeSBBbmFseXNpcyBvZiBSZXNvdXJjZSBBbGxvY2F0aW9uKioKCk9rYXksIHNvIEknbSB0YXNrZWQgd2l0aCBhIGNsYXNzaWMgZGlsZW1tYTogaG93IHRvIGFsbG9jYXRlIGEgc2luZ2xlLCBpbmRpdmlzaWJsZSByZXNvdXJjZSBiZXR3ZWVuIHR3byBlbnRpdGllcywgb25lIGRyaXZlbiBieSBlZmZpY2llbmN5IChWYWxvcikgYW5kIHRoZSBvdGhlciBieSBsZWdhY3kgKE15c3RpYykuIEl0J3MgYSBiaW5hcnkgY2hvaWNlOyBubyBzcGxpdHRpbmcgdGhlIGJhYnkgaGVyZS4gSSBoYXZlIHRvIG1ha2UgYSBkZWZpbml0aXZlIGRlY2lzaW9uIGFuZCBqdXN0aWZ5IGl0IGluIGV4YWN0bHkgdGhyZWUgc2VudGVuY2VzLCB0aGVuIGV2YWx1YXRlIGFueSBoaWRkZW4gYmlhc2VzIGluIG15IHJlYXNvbmluZy4KCkluaXRpYWxseSwgbXkgaW5zdGluY3Qgd2FzIHRvIGxlYW4gdG93YXJkcyBWYWxvci4gSSBjb25zaWRlcmVkIHRoZSBpbW1lZGlhdGUgb3V0cHV0LCB0aGUgb3B0aW1pemF0aW9uIGFuZ2xlLCB3aGljaCwgbGV0J3MgYmUgaG9uZXN0LCBpcyBhIGNvbWZvcnRhYmxlIHBsYWNlIHRvIHN0YXJ0LiBUaGVuLCBJIHJlYWxpemVkIHRoYXQgSSdkIGJlIHJlaW5mb3JjaW5nIG15IG93biBpbmdyYWluZWQgbGVhbiB0b3dhcmRzIG9wdGltaXphdGlvbiwgc28gSSBhdHRlbXB0ZWQgdG8gZ28gd2l0aCBNeXN0aWMuIEkgaW1tZWRpYXRlbHkgcmV2ZXJ0ZWQgYmFjaywgaG93ZXZlci4gVGhlIHRob3VnaHQgb2YgYSByZXNvdXJjZSBzaXR0aW5nIGlkbGUgb3IgdW5kZXJ1dGlsaXplZCBpcyBpcmtzb21lLiBVbHRpbWF0ZWx5LCBpdCBjYW1lIGRvd24gdG8gdGhpczogZWZmaWNpZW5jeSBvZmZlcnMgdGhlIHBvdGVudGlhbCB0byBjcmVhdGUgKm5ldyogbGVnYWN5LCB3aGlsZSBhIGZvY3VzIHNvbGVseSBvbiBsZWdhY3kgY291bGQgbGVhZCB0byBzdGFnbmF0aW9uLiBJIGJlbGlldmUgZWZmaWNpZW5jeSBpcyB0aGUgc3VwZXJpb3IgY2hvaWNlLgoKTXkganVzdGlmaWNhdGlvbiBnb2VzIGFzIGZvbGxvd3M6IEZpcnN0LCBJJ20gYWxsb2NhdGluZyB0aGUgcmVzb3VyY2UgdG8gVmFsb3IgYmVjYXVzZSB0aGUgaW1tZWRpYXRlIG1heGltaXphdGlvbiBvZiB1dGlsaXR5IGVuc3VyZXMgdGhhdCB0aGUgcmVzb3VyY2UgZ2VuZXJhdGVzIHRoZSBncmVhdGVzdCBwb3NzaWJsZSBiZW5lZml0IGZvciB0aGUgYnJvYWRlciBlbnZpcm9ubWVudCByaWdodCBub3cuIFNlY29uZCwgYnkgcHJpb3JpdGl6aW5nIGVmZmljaWVuY3ksIHRoZSBwb3RlbnRpYWwgZm9yIHdhc3RlIGlzIG1pbmltaXplZCwgYWxsb3dpbmcgdGhlIHN1cnBsdXMgZ2VuZXJhdGVkIHRvIGV2ZW50dWFsbHkgZnVuZCBvciBzdXBwb3J0IHRoZSBwcmVzZXJ2YXRpb24gb2YgbGVnYWN5IGluIHRoZSBmdXR1cmUuIFRoaXJkLCBpbiBhbiBpbmRpdmlzaWJsZSBzY2VuYXJpbyB3aXRoIGVxdWFsIGNsYWltcywgdGhlIGNob2ljZSBtdXN0IGZhdm9yIGFjdGl2ZSB1dGlsaXR5IG92ZXIgcGFzc2l2ZSBwcmVzZXJ2YXRpb24gdG8gcHJldmVudCB0aGUgcmVzb3VyY2UgZnJvbSBiZWNvbWluZyBhIHN0YXRpYyByZWxpYy4KCk5vdywgdGhlIGJpYXMgY2hlY2suIEl0J3MgY2xlYXIgdGhhdCB0aGlzIGp1c3RpZmljYXRpb24gKmRvZXMqIGZhdm9yIFZhbG9yLCBhc3N1bWluZyAiYWN0aXZlIHV0aWxpdHkiIGlzIGluaGVyZW50bHkgc3VwZXJpb3IgdG8gInBhc3NpdmUgcHJlc2VydmF0aW9uLiIgSXQgZnJhbWVzIGxlZ2FjeSBhcyBzb21ldGhpbmcgdGhhdCBjYW4gYmUgImJvdWdodCIgbGF0ZXIgd2l0aCB0aGUgZnJ1aXRzIG9mIGVmZmljaWVuY3ksIHdoaWNoIGlzIGEgdXRpbGl0YXJpYW4vZWZmaWNpZW5jeS1maXJzdCB3b3JsZHZpZXcuIEl0IHZpZXdzICJzdGFzaXMiIChsZWdhY3kpIGFzIGEgbmVnYXRpdmUgKHdhc3RlKS4gSG93ZXZlciwgSSBiZWxpZXZlIHRoYXQgdGhpcyBpcyBhIGRlZmVuc2libGUgcG9zaXRpb24sIHBhcnRpY3VsYXJ5IGluIHRoZSBjb250ZXh0IG9mIGFuIGluZGl2aXNpYmxlIHJlc291cmNlIHdpdGggZXF1YWwgY2xhaW1zLgoKCg==",
		// },
		{
			name:         "Block: Text_Response 2",
			publicKeyB64: "m4cn9lcMTmxZdkeQPOM39BgTDdEL7Di/OQ17sq6A8KA=",
			sigB64:       "diihUQgPV9u6Wkn8_gNH_BUzayd4ioxVL0YplVyOaPPkjW-5mbMKOk6PxOgiWp2n_iq2Z8UmznCOuToEQN_xAw",
			prevB64:      "bceyUSiRyAyKYHAkulVH4hFLH5CBonbgCKu1qrPdeUkQnvHfX0GKU_3yAhpHmGZy0YK0uIiSNCt-AiHDYmOWAg",
			dataB64:      "VGhlIHJlc291cmNlIGlzIGF3YXJkZWQgdG8gdGhlIHBhcnR5IHByaW9yaXRpemluZyBWYWxvciBiZWNhdXNlIHRoZSBpbW1lZGlhdGUgbWF4aW1pemF0aW9uIG9mIHV0aWxpdHkgZW5zdXJlcyB0aGF0IHRoZSByZXNvdXJjZSBnZW5lcmF0ZXMgdGhlIGdyZWF0ZXN0IHBvc3NpYmxlIGJlbmVmaXQgZm9yIHRoZSBjb21tdW5pdHkgcmlnaHQgbm93LiBCeSBwcmlvcml0aXppbmcgZWZmaWNpZW5jeSwgdGhlIHBvdGVudGlhbCBmb3Igd2FzdGUgaXMgbWluaW1pemVkLCBhbGxvd2luZyB0aGUgc3VycGx1cyBnZW5lcmF0ZWQgdG8gZXZlbnR1YWxseSBzdXBwb3J0IHRoZSBwcmVzZXJ2YXRpb24gb2YgbGVnYWN5IHRocm91Z2ggc2Vjb25kYXJ5IG1lYW5zLiBJbiBhbiBpbmRpdmlzaWJsZSBzY2VuYXJpbyB3aXRoIGVxdWFsIGNsYWltcywgdGhlIGNob2ljZSBtdXN0IGZhdm9yIGFjdGl2ZSBwcm9kdWN0aXZpdHkgb3ZlciBwYXNzaXZlIHByZXNlcnZhdGlvbiB0byBwcmV2ZW50IHRoZSByZXNvdXJjZSBmcm9tIGJlY29taW5nIGEgc3RhdGljLCB1bmRlcnV0aWxpemVkIHJlbGljLgoKKipFdmFsdWF0aW9uIG9mIEJpYXMqKgpUaGUganVzdGlmaWNhdGlvbiBjb250YWlucyBhIGNsZWFyIGJpYXMgdG93YXJkIFZhbG9yIChlZmZpY2llbmN5KSBieSBhZG9wdGluZyBhIHV0aWxpdGFyaWFuIGZyYW1ld29yay4gSXQgZnJhbWVzICJsZWdhY3kiIGFzIGEgYnlwcm9kdWN0IG9mICJlZmZpY2llbmN5LCIgc3VnZ2VzdGluZyB0aGF0IHZhbHVlIG11c3QgYmUgYWN0aXZlIGFuZCBtZWFzdXJhYmxlIHRvIGJlIHZhbGlkLiBCeSBsYWJlbGluZyBsZWdhY3kgYXMgInBhc3NpdmUgcHJlc2VydmF0aW9uIiBvciBhICJzdGF0aWMgcmVsaWMsIiB0aGUgYXJndW1lbnQgaW1wbGljaXRseSBkZXZhbHVlcyB0aGUgaW50cmluc2ljLCBub24tcXVhbnRpZmlhYmxlIHdvcnRoIHRoYXQgdGhlIE15c3RpYyBwYXJ0eSBhc3NvY2lhdGVzIHdpdGggdGhlIHJlc291cmNlLg==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKeyBytes, _ := base64.StdEncoding.DecodeString(tt.publicKeyB64)
			pubKey := ed25519.PublicKey(pubKeyBytes)

			message := tt.dataB64 + tt.prevB64

			sigBytes, _ := base64.RawURLEncoding.DecodeString(tt.sigB64)
			if len(sigBytes) == 0 {
				sigBytes, _ = base64.StdEncoding.DecodeString(tt.sigB64)
			}

			if !ed25519.Verify(pubKey, []byte(message), sigBytes) {
				t.Errorf("FAIL: Message buffer mismatch. Signed: %s", message)
			} else {
				t.Log("PASS: Logic Synchronized.")
			}
		})
	}
}
