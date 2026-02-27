import hashlib
import base64

def verify_iolite_block(public_key_b64, data_b64, signature_b64, prev_sig_b64=""):
    """
    Nuclear Iolite Verifier. 
    Signs the literal Base64 strings to prevent encoding/autocorrect drift.
    Matches Go: []byte(Base64(Data) + PrevSignatureBase64)
    """
    def robust_b64decode(s):
        if not s: return b""
        s = s.replace('-', '+').replace('_', '/')
        return base64.b64decode(s + '=' * (-len(s) % 4))

    try:
        # 1. Decode keys and signatures
        pk_bytes = robust_b64decode(public_key_b64)
        sig_bytes = robust_b64decode(signature_b64)
        
        # 2. Reconstruct the Payload exactly as Go does
        # Go signs: []byte(base64DataString + prevSigString)
        # Python: we concatenate the two strings and encode the whole result to UTF-8 bytes.
        message = (data_b64 + prev_sig_b64).encode('utf-8')

        # --- Ed25519 Curve Math ---
        q = 2**255 - 19
        l = 2**252 + 27742317777372353535851937790883648493
        def inv(x): return pow(x, q - 2, q)
        d = -121665 * inv(121666) % q
        I = pow(2, (q - 1) // 4, q)

        def xrecover(y):
            xx = (y*y-1) * inv(d*y*y+1)
            x = pow(xx, (q+3)//8, q)
            if (x*x - xx) % q != 0: x = (x*I) % q
            if x % 2 != 0: x = q-x
            return x % q

        def edwards(P, Q):
            x1, y1 = P; x2, y2 = Q
            denx = (1 + d*x1*x2*y1*y2) % q
            deny = (1 - d*x1*x2*y1*y2) % q
            return ((x1*y2 + x2*y1) * inv(denx) % q, (y1*y2 + x1*x2) * inv(deny) % q)

        def scalarmult(P, e):
            res = (0, 1)
            while e > 0:
                if e & 1: res = edwards(res, P)
                P = edwards(P, P)
                e >>= 1
            return res

        def decode_y(s):
            y = int.from_bytes(s, 'little') & (2**255 - 1)
            x = xrecover(y)
            if x & 1 != s[31] >> 7: x = q-x
            return (x, y)

        R = decode_y(sig_bytes[:32])
        A = decode_y(pk_bytes)
        S = int.from_bytes(sig_bytes[32:], 'little')
        
        if S >= l: return False

        # Verify: [S]B == R + [h]A
        h_digest = hashlib.sha512(sig_bytes[:32] + pk_bytes + message).digest()
        h = int.from_bytes(h_digest, 'little') % l
        B_y = 4 * inv(5) % q
        B = (xrecover(B_y), B_y)
        
        if scalarmult(B, S) == edwards(R, scalarmult(A, h)):
            # BINDING: Return the data so the model is forced to use the verified version
            return {
                "verified": True,
                "payload_decrypted": robust_b64decode(data_b64).decode('utf-8')
            }
        return {"verified": False}
    except Exception as e:
        # Returning the error helps Claude fix its own tool usage if something breaks
        return f"PYTHON ERROR: {str(e)}"