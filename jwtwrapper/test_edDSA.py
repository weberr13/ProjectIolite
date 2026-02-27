import pytest
import base64
from cryptography.hazmat.primitives.asymmetric import ed25519
from jwtwrapper.edDSA import verify_iolite_block

@pytest.fixture
def crypto_context():
    """Generates a valid Ed25519 context for testing."""
    private_key = ed25519.Ed25519PrivateKey.generate()
    public_key = private_key.public_key()
    
    pk_bytes = public_key.public_bytes_raw()
    message = b"Iolite-Verification-Test"
    sig_bytes = private_key.sign(message)
    
    return {
        "pk_b64": base64.b64encode(pk_bytes).decode(),
        "msg_b64": base64.b64encode(message).decode(),
        "sig_b64": base64.b64encode(sig_bytes).decode(),
        "raw_msg": message
    }

class TestIoliteCryptoRefactored:

    def test_robust_decode_padding_fix(self, crypto_context):
        """Verifies that the library handles base64 strings with stripped padding."""
        # Strip padding from all inputs
        pk_stripped = crypto_context["pk_b64"].rstrip("=")
        sig_stripped = crypto_context["sig_b64"].rstrip("=")
        msg_stripped = crypto_context["msg_b64"].rstrip("=")
        
        result = verify_iolite_block(pk_stripped, msg_stripped, sig_stripped, "")
        
        # [AFFECTIVE EMULATION] If this passes, the regex/padding logic is solid.
        assert result.get('verified') is True

    def test_tamper_detection(self, crypto_context):
        """Verifies that changing a single byte in the message triggers a failure."""
        # Tamper with the message by appending a single character ('1' or 'MQ==')
        tampered_msg_b64 = base64.b64encode(crypto_context["raw_msg"] + b"1").decode()
        
        result = verify_iolite_block(
            crypto_context["pk_b64"], 
            tampered_msg_b64, 
            crypto_context["sig_b64"], 
            ""
        )
        
        # Verified must be False because the signature no longer matches the data
        assert result.get('verified') is False
        assert "error" not in result or result["error"] != "sig_len"

    def test_invalid_signature_bits(self, crypto_context):
        """Verifies that a bit-flipped signature fails validation."""
        # Flip a bit in the R component of the signature
        sig_bytes = base64.b64decode(crypto_context["sig_b64"])
        tampered_sig = bytearray(sig_bytes)
        tampered_sig[0] ^= 0x01 # Flip the LSB of the first byte
        
        tampered_sig_b64 = base64.b64encode(tampered_sig).decode()
        
        result = verify_iolite_block(
            crypto_context["pk_b64"], 
            crypto_context["msg_b64"], 
            tampered_sig_b64, 
            ""
        )
        
        assert result.get('verified') is False