import { useEffect, useMemo, useState, type ReactNode } from 'react'

import { fetchMe, type AuthUser } from '../api'
import { AuthContext, type AuthContextValue } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)

  // No session to persist client-side: identity comes from the Remote-Email
  // header Pangolin sets on every request, so each page load asks the API
  // again instead of trusting anything stored locally.
  useEffect(() => {
    let cancelled = false
    fetchMe()
      .then((me) => {
        if (!cancelled) setUser(me)
      })
      .catch(() => {
        if (!cancelled) setUser(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      role: user?.role ?? 'student',
      isAuthenticated: user !== null,
      isTeacher: user !== null && user.role === 'teacher',
      loading,
    }),
    [user, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
