import pytest
import base64
from cryptography.hazmat.primitives.asymmetric import ed25519
from jwtwrapper.edDSA import verify_iolite_block

@pytest.fixture
def crypto_context():
    """Generates a valid Ed25519 context for testing 'Sign-the-String' logic."""
    private_key = ed25519.Ed25519PrivateKey.generate()
    public_key = private_key.public_key()
    
    pk_bytes = public_key.public_bytes_raw()
    
    # [REALITY CHECK]: In Iolite, we sign the B64 representation of the data.
    raw_content = "Iolite-Verification-Test"
    data_b64 = base64.b64encode(raw_content.encode('utf-8')).decode('utf-8')
    prev_sig_b64 = "SomePrevSignatureString"
    
    # The message the Signer actually sees is the concatenated ASCII strings
    signing_message = (data_b64 + prev_sig_b64).encode('utf-8')
    sig_bytes = private_key.sign(signing_message)
    
    return {
        "pk_b64": base64.b64encode(pk_bytes).decode(),
        "data_b64": data_b64,
        "prev_sig_b64": prev_sig_b64,
        "sig_b64": base64.b64encode(sig_bytes).decode(),
        "raw_content": raw_content
    }

class TestIoliteCryptoRefactored:

    def test_iolite_string_verification(self, crypto_context):
        """Verifies that the block matches the Go 'Sign-the-String' pattern."""
        result = verify_iolite_block(
            crypto_context["pk_b64"], 
            crypto_context["data_b64"], 
            crypto_context["sig_b64"], 
            crypto_context["prev_sig_b64"]
        )
        
        assert result.get('verified') is True
        assert result.get('payload_decrypted') == crypto_context["raw_content"]

    def test_robust_decode_padding_fix(self, crypto_context):
        """Verifies that cryptography values handle stripped padding, but data_b64 is literal."""
        pk_stripped = crypto_context["pk_b64"].rstrip("=")
        sig_stripped = crypto_context["sig_b64"].rstrip("=")
        
        # Note: We do NOT strip data_b64 because that would change the signed message bytes!
        result = verify_iolite_block(
            pk_stripped, 
            crypto_context["data_b64"], 
            sig_stripped, 
            crypto_context["prev_sig_b64"]
        )
        assert result.get('verified') is True    

    def test_tamper_detection_in_string(self, crypto_context):
        """Verifies that changing a single character in the B64 string fails verification."""
        # Flip a character in the data_b64 string
        tampered_data = crypto_context["data_b64"][:-1] + ("0" if crypto_context["data_b64"][-1] != "0" else "1")
        
        result = verify_iolite_block(
            crypto_context["pk_b64"], 
            tampered_data, 
            crypto_context["sig_b64"], 
            crypto_context["prev_sig_b64"]
        )
        
        assert result.get('verified') is False

    def test_genesis_anchor_verification(self, crypto_context):
        """Verifies that an empty prev_sig (Genesis) works correctly."""
        # Setup a genesis block
        private_key = ed25519.Ed25519PrivateKey.generate()
        data_b64 = base64.b64encode(b"Genesis-Block").decode('utf-8')
        
        # Message is just the data_b64 + ""
        sig_bytes = private_key.sign(data_b64.encode('utf-8'))
        
        result = verify_iolite_block(
            base64.b64encode(private_key.public_key().public_bytes_raw()).decode(),
            data_b64,
            base64.b64encode(sig_bytes).decode(),
            ""
        )
        assert result.get('verified') is True
