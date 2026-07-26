import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { AuthProvider } from './auth/AuthContext'
import { RequireAuth } from './auth/RequireAuth'
import { AppLayout } from './components/AppLayout'
import { ChallengeList } from './pages/ChallengeList'
import { ChallengePage } from './pages/ChallengePage'
import { ClassboardPage } from './pages/ClassboardPage'
import { ListDetailPage } from './pages/ListDetailPage'
import { ListsPage } from './pages/ListsPage'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { SubmissionHistoryPage } from './pages/SubmissionHistoryPage'
import { SubmissionPage } from './pages/SubmissionPage'

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/" element={<ChallengeList />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
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
              path="/listas"
              element={
                <RequireAuth>
                  <ListsPage />
                </RequireAuth>
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
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App
