import { base64UrlDecode, getTokenExp, isExpiredOrExpiring } from './jwt';

// Buffer exists in the Jest (Node) runtime; declare it to satisfy the
// React Native tsconfig, which does not pull in @types/node.
declare const Buffer: {
  from(data: string): { toString(encoding: string): string };
};

// Builds a base64url-encoded JWT (signature is irrelevant for these helpers).
function makeJwt(payload: Record<string, unknown>): string {
  const enc = (obj: Record<string, unknown>) =>
    Buffer.from(JSON.stringify(obj))
      .toString('base64')
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
  return `${enc({ alg: 'HS256', typ: 'JWT' })}.${enc(payload)}.sig`;
}

describe('base64UrlDecode', () => {
  it('decodes base64url without relying on atob', () => {
    const json = '{"sub":"abc","exp":123}';
    const b64url = Buffer.from(json)
      .toString('base64')
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    expect(base64UrlDecode(b64url)).toBe(json);
  });
});

describe('getTokenExp', () => {
  it('extracts the exp claim', () => {
    expect(getTokenExp(makeJwt({ exp: 1781701939 }))).toBe(1781701939);
  });

  it('returns 0 for a malformed token', () => {
    expect(getTokenExp('garbage')).toBe(0);
  });

  it('returns 0 when exp is absent', () => {
    expect(getTokenExp(makeJwt({ sub: 'x' }))).toBe(0);
  });
});

describe('isExpiredOrExpiring', () => {
  const now = Math.floor(Date.now() / 1000);

  it('returns false for a token valid well beyond the buffer', () => {
    expect(isExpiredOrExpiring(makeJwt({ exp: now + 3600 }))).toBe(false);
  });

  it('returns true for an already-expired token', () => {
    expect(isExpiredOrExpiring(makeJwt({ exp: now - 10 }))).toBe(true);
  });

  it('returns true within the 60s expiry buffer', () => {
    expect(isExpiredOrExpiring(makeJwt({ exp: now + 30 }))).toBe(true);
  });

  it('returns true for a malformed token', () => {
    expect(isExpiredOrExpiring('not-a-jwt')).toBe(true);
  });

  it('returns true when exp claim is missing', () => {
    expect(isExpiredOrExpiring(makeJwt({ sub: 'x' }))).toBe(true);
  });
});
