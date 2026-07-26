import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import {
  UNAUTHORIZED_EVENT,
  clearSession,
  getStoredUser,
  getToken,
  loginUser,
  registerUser,
  storeSession,
  type AuthResponse,
  type AuthUser,
} from '../api'
import { decodeTokenClaims, isTokenExpired, type UserRole } from './claims'
import { AuthContext, type AuthContextValue } from './context'

interface SessionState {
  user: AuthUser | null
  role: UserRole
}

const anonymous: SessionState = { user: null, role: 'student' }

// Validates the persisted token on boot: malformed or expired tokens are
// discarded instead of triggering a doomed request → 401 → redirect loop.
function initialSession(): SessionState {
  const token = getToken()
  if (token === null) {
    return anonymous
  }
  const claims = decodeTokenClaims(token)
  if (claims === null || isTokenExpired(claims)) {
    clearSession()
    return anonymous
  }
  return { user: getStoredUser(), role: claims.role }
}

function sessionFromResponse(response: AuthResponse): SessionState {
  const claims = decodeTokenClaims(response.token)
  return { user: response.user, role: claims?.role ?? 'student' }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState>(initialSession)

  const logout = useCallback(() => {
    clearSession()
    setSession(anonymous)
  }, [])

  // api.ts clears storage and fires this exactly once per authenticated
  // session when a 401 comes back; guards then redirect declaratively.
  useEffect(() => {
    const onUnauthorized = () => setSession(anonymous)
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const response = await loginUser(email, password)
    storeSession(response)
    setSession(sessionFromResponse(response))
  }, [])

  const register = useCallback(async (email: string, password: string) => {
    const response = await registerUser(email, password)
    storeSession(response)
    setSession(sessionFromResponse(response))
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      user: session.user,
      role: session.role,
      isAuthenticated: session.user !== null,
      isTeacher: session.user !== null && session.role === 'teacher',
      login,
      register,
      logout,
    }),
    [session, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
