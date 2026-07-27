import type { ReactNode } from 'react'

import { useAuth } from './useAuth'

// Route guard: identity comes from Pangolin (or DEV_FAKE_EMAIL locally),
// never from a login form, so there is nowhere to redirect an unidentified
// visitor to — show an inline message instead.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) {
    return null
  }
  if (!isAuthenticated) {
    return <p className="status-message">Sem acesso. Sua identidade não foi reconhecida.</p>
  }
  return <>{children}</>
}

// Teacher-only guard for administrative surfaces. UI gating only — the API
// must still enforce authorization server-side.
export function RequireTeacher({ children }: { children: ReactNode }) {
  const { isAuthenticated, isTeacher, loading } = useAuth()

  if (loading) {
    return null
  }
  if (!isAuthenticated || !isTeacher) {
    return <p className="status-message">Sem acesso. Esta área é restrita a professores.</p>
  }
  return <>{children}</>
}
