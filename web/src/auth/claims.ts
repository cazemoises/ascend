export type UserRole = 'student' | 'teacher'

export interface TokenClaims {
  sub: string
  email: string
  exp: number
  role: UserRole
}

// Decodes the JWT payload client-side for UX decisions only (role-based UI,
// expiry checks). The server remains the authority — it verifies signatures.
export function decodeTokenClaims(token: string): TokenClaims | null {
  const parts = token.split('.')
  if (parts.length !== 3) {
    return null
  }
  try {
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    const payload = JSON.parse(atob(padded)) as Record<string, unknown>

    const sub = typeof payload.sub === 'string' ? payload.sub : null
    const email = typeof payload.email === 'string' ? payload.email : null
    const exp = typeof payload.exp === 'number' ? payload.exp : null
    if (sub === null || email === null || exp === null) {
      return null
    }
    return {
      sub,
      email,
      exp,
      role: payload.role === 'teacher' ? 'teacher' : 'student',
    }
  } catch {
    return null
  }
}

export function isTokenExpired(claims: TokenClaims, skewSeconds = 30): boolean {
  return Date.now() / 1000 >= claims.exp - skewSeconds
}
