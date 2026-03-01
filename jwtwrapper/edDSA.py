import hashlib
import base64

# 1. Move B to the Immutable Constants block at the top
# These are the bit-exact coordinates for the Ed25519 Generator Point B
B_X = 15112221349535400772501151409588531511454012693041857206046113283949847762202
B_Y = 46316835694926478169428394003475163141307993866256225615783033603165251855960
G_POINT = (B_X, B_Y)

def verify_iolite_block(public_key_b64, data_b64, signature_b64, prev_sig_b64=""):
    def robust_decode(s):
        if not s: return b""
        s = "".join(s.replace('-', '+').replace('_', '/').split())
        return base64.b64decode(s + '=' * (-len(s) % 4))

    # IMMUTABLE CONSTANTS
    L_ORDER = 2**252 + 27742317777372353535851937790883648493
    P_CURVE = 2**255 - 19
    inv = lambda x: pow(x, P_CURVE - 2, P_CURVE)
    # Ed25519 d = -121665/121666 mod p
    D_CONST = (-121665 * inv(121666)) % P_CURVE
    # I = sqrt(-1) mod p
    I_CONST = pow(2, (P_CURVE - 1) // 4, P_CURVE)    

    try:
        pk_bytes = robust_decode(public_key_b64)
        sig_bytes = robust_decode(signature_b64)
        if len(sig_bytes) != 64 or len(pk_bytes) != 32:
            return {"verified": False, "error": "sig_len"}

        # # Binary message reconstruction
        # m1 = robust_decode(data_b64) if data_b64 else b""
        # m2 = robust_decode(prev_sig_b64) if prev_sig_b64 else b""

        # FIX: Reconstruct message using literal string bytes
        # This matches Go's `b64Data + s.PrevSignature`
        m1 = data_b64.encode('utf-8') if data_b64 else b""
        m2 = prev_sig_b64.encode('utf-8') if prev_sig_b64 else b""
        message = m1 + m2

        inv = lambda x: pow(x, P_CURVE - 2, P_CURVE)

        def xrecover(label, y):
            y2 = (y * y) % P_CURVE
            u = (y2 - 1) % P_CURVE
            v = (D_CONST * y2 + 1) % P_CURVE

            v2 = (v * v) % P_CURVE
            v3 = (v2 * v) % P_CURVE
            v4 = (v2 * v2) % P_CURVE
            v7 = (v4 * v3) % P_CURVE

            # GROUND TRUTH ASSEMBLY
            base = (u * v7) % P_CURVE
            exp = 7237005577332262213973186563042994240829374041602535252466099000494570602493

            candidate = pow(base, exp, P_CURVE)

            # x = u * v^3 * candidate
            x = (u * v3) % P_CURVE
            x = (x * candidate) % P_CURVE
                
            # Check v * x^2
            x2 = (x * x) % P_CURVE

            vx2 = (v * x2) % P_CURVE
            if vx2 != u:
                if vx2 == (P_CURVE - u) % P_CURVE:
                    x = (x * I_CONST) % P_CURVE
                else:
                    # If this triggers with our verified v, 
                    # there is a logic error in the exponentiation chain.
                    raise ValueError(f"{label} root failure")
            
            return x % P_CURVE

        def decode_point(label, s):
            # s is 32 bytes. Ed25519: y is bits 0-254. Bit 255 is the x-parity.
            # We need the full integer to extract the parity correctly.
            y_int = int.from_bytes(s, 'little')
            
            # The actual y coordinate is y_int with the 255th bit cleared
            y = y_int & (2**255 - 1)
            x = xrecover(label, y)
            
            # RFC 8032: The 255th bit of the last byte is the parity of x
            expected_parity = (s[31] >> 7) & 1
            if (x & 1) != expected_parity:
                x = P_CURVE - x

            # Curve verification: -x^2 + y^2 = 1 + dx^2y^2
            x2 = (x * x) % P_CURVE
            lhs = (y * y - x2) % P_CURVE
            rhs = (1 + D_CONST * x2 * y * y) % P_CURVE
            
            if lhs != rhs:
                # If this prints now, the u/v logic in xrecover is still 
                # hitting that exponent underflow.
                raise ValueError(f"{label} root failure: not on a curve")

            return (x % P_CURVE, y % P_CURVE)

        def edwards_add(P, Q):
            x1, y1 = P[0], P[1]
            x2, y2 = Q[0], Q[1]
            
            # Common term used in both denominators
            d_x1x2y1y2 = (D_CONST * x1 * x2 * y1 * y2) % P_CURVE
            
            # x3 = (x1y2 + x2y1) / (1 + dx1x2y1y2)
            # y3 = (y1y2 + x1x2) / (1 - dx1x2y1y2)
            x3 = ((x1 * y2 + x2 * y1) * inv(1 + d_x1x2y1y2)) % P_CURVE
            y3 = ((y1 * y2 + x1 * x2) * inv(1 - d_x1x2y1y2)) % P_CURVE
            
            return (x3, y3)

        def scalarmult(P, e):
            res = (0, 1)
            while e > 0:
                if e & 1: res = edwards_add(res, P)
                P = edwards_add(P, P)
                e >>= 1
            return res

        R = decode_point("R", sig_bytes[:32])
        A = decode_point("A", pk_bytes)
        S = int.from_bytes(sig_bytes[32:], 'little')
        if S >= L_ORDER:
            return {"verified": False, "error": "S_out_of_range"}
        
        # --- THE FIX: REMOVE REDUCTION ---
        h_digest = hashlib.sha512(sig_bytes[:32] + pk_bytes + message).digest()
        # RFC 8032: h is the full 512-bit integer. DO NOT REDUCE mod l.
        h = int.from_bytes(h_digest, 'little') 
        # Reducing h mod L is mathematically equivalent for the scalarmult
        h_reduced = h

        scmult = scalarmult(G_POINT, S)
        rhs = edwards_add(R, scalarmult(A, h_reduced))
        
        if scmult == rhs:
            # We still decode for the BTU return, but NOT for the signature check
            actual_payload = base64.b64decode(data_b64).decode('utf-8')
            return {"verified": True, "payload_decrypted": actual_payload}
        
        return {
            "verified": False, 
            "scalarmult": {scmult[0].to_bytes(32, byteorder='little').hex(), scmult[1].to_bytes(32, byteorder='little').hex()}, 
            "edwards_add": {rhs[0].to_bytes(32, byteorder='little').hex(), rhs[1].to_bytes(32, byteorder='little').hex()}, 
            "S":  S.to_bytes(32, byteorder='little').hex(), 
            "R": {R[0].to_bytes(32, byteorder='little').hex(), R[1].to_bytes(32, byteorder='little').hex()}, 
            "A": {A[0].to_bytes(32, byteorder='little').hex(), A[1].to_bytes(32, byteorder='little').hex()}
        }
    except Exception as e:
        return {"verified": False, "error": str(e)}