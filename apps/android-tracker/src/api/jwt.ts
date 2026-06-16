const B64_ALPHABET =
  'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';

// Pure-JS base64url decoder. We do NOT rely on global atob: it is missing or
// broken on Hermes for some RN versions, and a throwing atob would make every
// token look "expired" and trigger an endless refresh of single-use refresh
// tokens (race → 401 → spurious logout).
export function base64UrlDecode(input: string): string {
  const str = input.replace(/-/g, '+').replace(/_/g, '/');
  let output = '';
  let buffer = 0;
  let bits = 0;
  for (const ch of str) {
    if (ch === '=') break;
    const val = B64_ALPHABET.indexOf(ch);
    if (val === -1) continue;
    buffer = (buffer << 6) | val;
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      output += String.fromCharCode((buffer >> bits) & 0xff);
    }
  }
  return output;
}

// Returns true if the JWT is missing, unparsable, or expires within bufferSec.
export function isExpiredOrExpiring(token: string, bufferSec = 60): boolean {
  try {
    const payloadB64 = token.split('.')[1];
    if (!payloadB64) return true;
    const payload = JSON.parse(base64UrlDecode(payloadB64)) as { exp?: number };
    return (payload.exp ?? 0) - Date.now() / 1000 < bufferSec;
  } catch {
    return true;
  }
}
