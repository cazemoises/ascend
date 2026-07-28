import { createContext } from 'react'

import type { AuthUser, UserRole } from '../api'

export interface AuthContextValue {
  user: AuthUser | null
  // Effective role (post "Ver como aluno" override, if any) — what every UI
  // gate other than the toggle itself should check.
  role: UserRole
  isAuthenticated: boolean
  isTeacher: boolean
  // Real role, ignoring the override — only ever used to decide whether the
  // "Ver como aluno" toggle itself is shown.
  isRealTeacher: boolean
  viewingAsStudent: boolean
  setViewingAsStudent: (value: boolean) => void
  // True until the initial GET /auth/me resolves — lets route guards avoid
  // flashing "no access" before the real identity is known.
  loading: boolean
}

export const AuthContext = createContext<AuthContextValue | null>(null)
