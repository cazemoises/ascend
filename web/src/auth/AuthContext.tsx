import { useEffect, useMemo, useState, type ReactNode } from 'react'

import { fetchMe, getViewAsStudent, setViewAsStudent, type AuthUser } from '../api'
import { AuthContext, type AuthContextValue } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [viewingAsStudent, setViewingAsStudentState] = useState(getViewAsStudent)

  // No session to persist client-side: identity comes from the Remote-Email
  // header Pangolin sets on every request, so each page load asks the API
  // again instead of trusting anything stored locally. Re-fetched whenever
  // the "Ver como aluno" toggle changes, since that changes the X-View-As
  // header requestJSON sends and therefore effective_role in the response.
  useEffect(() => {
    let cancelled = false
    setLoading(true)
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
  }, [viewingAsStudent])

  function updateViewingAsStudent(next: boolean) {
    setViewAsStudent(next)
    setViewingAsStudentState(next)
  }

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      role: user?.effective_role ?? 'student',
      isAuthenticated: user !== null,
      isTeacher: user !== null && user.effective_role === 'teacher',
      isRealTeacher: user !== null && user.real_role === 'teacher',
      viewingAsStudent,
      setViewingAsStudent: updateViewingAsStudent,
      loading,
    }),
    [user, loading, viewingAsStudent],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
