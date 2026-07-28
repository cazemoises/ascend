import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import { AuthProvider } from './auth/AuthContext'
import { RequireAuth, RequireTeacher } from './auth/RequireAuth'
import { AppLayout } from './components/AppLayout'
import { ChallengeList } from './pages/ChallengeList'
import { ChallengePage } from './pages/ChallengePage'
import { ListDetailPage } from './pages/ListDetailPage'
import { ListFormPage } from './pages/ListFormPage'
import { ListsPage } from './pages/ListsPage'
import { StudentsOverviewPage } from './pages/StudentsOverviewPage'
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

            {/* Rotas das Listas (Sempre Ativas, leitura sem login) */}
            <Route path="/listas" element={<ListsPage />} />
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
            <Route path="/listas/:id" element={<ListDetailPage />} />

            {/* Progresso dos alunos: sempre ativo, engloba desafios e listas */}
            <Route
              path="/progresso"
              element={
                <RequireTeacher>
                  <StudentsOverviewPage />
                </RequireTeacher>
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