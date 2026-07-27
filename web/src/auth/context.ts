import { createContext } from 'react'

import type { AuthUser, UserRole } from '../api'

export interface AuthContextValue {
  user: AuthUser | null
  role: UserRole
  isAuthenticated: boolean
  isTeacher: boolean
  // True until the initial GET /auth/me resolves — lets route guards avoid
  // flashing "no access" before the real identity is known.
  loading: boolean
}

export const AuthContext = createContext<AuthContextValue | null>(null)
