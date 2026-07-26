import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { useAuth } from './useAuth'

// Route guard: unauthenticated visitors are sent to /login exactly once,
// remembering where they were headed so LoginPage can return them there.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  const location = useLocation()

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}

// Teacher-only guard for administrative surfaces. UI gating only — the API
// must still enforce authorization server-side.
export function RequireTeacher({ children }: { children: ReactNode }) {
  const { isAuthenticated, isTeacher } = useAuth()
  const location = useLocation()

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  if (!isTeacher) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}
