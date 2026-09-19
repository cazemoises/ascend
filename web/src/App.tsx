import { BrowserRouter, Navigate, Route, Routes, useParams } from 'react-router-dom'

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
import { LiveRoomPage } from './pages/LiveRoomPage'
import { LiveSessionsPage } from './pages/LiveSessionsPage'

const isOnlyLists = import.meta.env.VITE_ONLY_LISTS_MODE === 'true'
console.log('VITE_ONLY_LISTS_MODE:', isOnlyLists)

// React Router keeps the same LiveRoomPage instance alive across a
// room-to-room navigation (only :id/:itemId change, not the matched
// element) — this key forces a full remount instead, so each room gets its
// own websocket/Y.Doc lifecycle rather than reusing a stale one.
function LiveRoomPageWithKey() {
  const { id, itemId } = useParams<{ id: string; itemId: string }>()
  return <LiveRoomPage key={`${id}-${itemId}`} />
}

function App() {
  return (
    <AuthProvider>
      <BrowserRouter basename={import.meta.env.BASE_URL}>
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
            <Route path="/sessoes" element={<RequireAuth><LiveSessionsPage /></RequireAuth>} />
            <Route path="/sessoes/:id" element={<RequireAuth><LiveSessionsPage /></RequireAuth>} />
            <Route path="/sessoes/:id/salas/:itemId" element={<RequireAuth><LiveRoomPageWithKey /></RequireAuth>} />

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
