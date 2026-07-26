import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import { AuthProvider } from './auth/AuthContext'
import { RequireAuth, RequireTeacher } from './auth/RequireAuth'
import { AppLayout } from './components/AppLayout'
import { ChallengeList } from './pages/ChallengeList'
import { ChallengePage } from './pages/ChallengePage'
import { ClassboardPage } from './pages/ClassboardPage'
import { ListDetailPage } from './pages/ListDetailPage'
import { ListFormPage } from './pages/ListFormPage'
import { ListsPage } from './pages/ListsPage'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { SubmissionHistoryPage } from './pages/SubmissionHistoryPage'
import { SubmissionPage } from './pages/SubmissionPage'

const isOnlyLists = import.meta.env.VITE_ONLY_LISTS_MODE === 'true'
console.log('VITE_ONLY_LISTS_MODE:', isOnlyLists)

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            {/* Redirecionamento da raiz dependendo do modo */}
            <Route
              path="/"
              element={
                isOnlyLists ? (
                  <Navigate to="/listas" replace />
                ) : (
                  <ChallengeList />
                )
              }
            />

            {/* Rotas Públicas */}
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />

            {/* Rotas das Listas (Sempre Ativas) */}
            <Route
              path="/listas"
              element={
                <RequireAuth>
                  <ListsPage />
                </RequireAuth>
              }
            />
            <Route
              path="/listas/nova"
              element={
                <RequireTeacher>
                  <ListFormPage />
                </RequireTeacher>
              }
            />
            <Route
              path="/listas/:id/editar"
              element={
                <RequireTeacher>
                  <ListFormPage />
                </RequireTeacher>
              }
            />
            <Route
              path="/listas/:id"
              element={
                <RequireAuth>
                  <ListDetailPage />
                </RequireAuth>
              }
            />

            {/* Rotas de Judge/Desafios (Desativadas se isOnlyLists = true) */}
            {!isOnlyLists && (
              <>
                <Route
                  path="/submissions"
                  element={
                    <RequireAuth>
                      <SubmissionHistoryPage />
                    </RequireAuth>
                  }
                />
                <Route
                  path="/turmas"
                  element={
                    <RequireAuth>
                      <ClassboardPage />
                    </RequireAuth>
                  }
                />
                <Route
                  path="/challenges/:id"
                  element={
                    <RequireAuth>
                      <ChallengePage />
                    </RequireAuth>
                  }
                />
                <Route
                  path="/challenges/:id/submissions/:subId"
                  element={
                    <RequireAuth>
                      <SubmissionPage />
                    </RequireAuth>
                  }
                />
              </>
            )}

            {/* Catch-all para rotas não mapeadas no modo exclusivo */}
            <Route
              path="*"
              element={<Navigate to={isOnlyLists ? '/listas' : '/'} replace />}
            />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App