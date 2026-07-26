import { createContext } from 'react'

import type { AuthUser } from '../api'
import type { UserRole } from './claims'

export interface AuthContextValue {
  user: AuthUser | null
  role: UserRole
  isAuthenticated: boolean
  isTeacher: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  logout: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)
