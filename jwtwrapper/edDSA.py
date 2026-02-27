import hashlib
import base64

def verify_iolite_block(public_key_b64, data, signature_b64, prev_sig_b64=""):
    """
    Standard Ed25519 Verification (RFC 8032).
    Fixed: Hash reduction modulo l and iterative scalar multiplication.
    """
    def robust_b64decode(s):
        s = s.replace('-', '+').replace('_', '/')
        return base64.b64decode(s + '=' * (-len(s) % 4))

    # Constants
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
        x3 = (x1*y2 + x2*y1) * inv(denx)
        y3 = (y1*y2 + x1*x2) * inv(deny)
        return (x3 % q, y3 % q)

    def scalarmult(P, e):
        res = (0, 1) # Neutral element
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

    try:
        pk_bytes = robust_b64decode(public_key_b64)
        sig_bytes = robust_b64decode(signature_b64)
        R = decode_y(sig_bytes[:32])
        A = decode_y(pk_bytes)
        S = int.from_bytes(sig_bytes[32:], 'little')
        
        if S >= l: return False # S must be in [0, l-1]

        # Chaining: Data + PrevSignature
        message = (data + prev_sig_b64).encode('utf-8')
        h_digest = hashlib.sha512(sig_bytes[:32] + pk_bytes + message).digest()
        h = int.from_bytes(h_digest, 'little') % l # Fixed: Full hash reduced mod l
        
        # Base point B (y=4/5)
        B_y = 4 * inv(5) % q
        B = (xrecover(B_y), B_y)
        
        # Verify: [S]B == R + [h]A
        return scalarmult(B, S) == edwards(R, scalarmult(A, h))
    except:
        return False