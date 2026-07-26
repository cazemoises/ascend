import { NavLink, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

function IconChallenges() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  )
}

function IconLists() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <line x1="8" y1="6" x2="21" y2="6" />
      <line x1="8" y1="12" x2="21" y2="12" />
      <line x1="8" y1="18" x2="21" y2="18" />
      <line x1="3" y1="6" x2="3.01" y2="6" />
      <line x1="3" y1="12" x2="3.01" y2="12" />
      <line x1="3" y1="18" x2="3.01" y2="18" />
    </svg>
  )
}

function IconHistory() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <polyline points="12 6 12 12 16 14" />
    </svg>
  )
}

function IconClasses() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  )
}

function IconLogin() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
      <polyline points="10 17 15 12 10 7" />
      <line x1="15" y1="12" x2="3" y2="12" />
    </svg>
  )
}

function IconLogout() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  )
}

const linkClass = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'sidebar__link sidebar__link--active' : 'sidebar__link'

export function Sidebar() {
  const { user, isAuthenticated, isTeacher, logout } = useAuth()
  const navigate = useNavigate()

  return (
    <aside className="sidebar">
      <div className="sidebar__brand">
        <span className="sidebar__brand-mark">▲</span>
        <span>Ascend</span>
      </div>

      <nav className="sidebar__nav" aria-label="Navegação principal">
        <NavLink to="/" end className={linkClass}>
          <IconChallenges />
          Desafios
        </NavLink>
        {isAuthenticated ? (
          <NavLink to="/listas" className={linkClass}>
            <IconLists />
            Listas
          </NavLink>
        ) : null}
        {isAuthenticated ? (
          <NavLink to="/submissions" className={linkClass}>
            <IconHistory />
            Histórico
          </NavLink>
        ) : null}
        {isTeacher ? (
          <NavLink to="/turmas" className={linkClass}>
            <IconClasses />
            Turmas
          </NavLink>
        ) : null}
      </nav>

      <div className="sidebar__footer">
        {isAuthenticated && user !== null ? (
          <>
            <div className="sidebar__user">
              <span className="sidebar__user-email" title={user.email}>
                {user.email}
              </span>
              <span className={isTeacher ? 'sidebar__role sidebar__role--teacher' : 'sidebar__role'}>
                {isTeacher ? 'Professor' : 'Estudante'}
              </span>
            </div>
            <button
              type="button"
              className="sidebar__logout"
              onClick={() => {
                logout()
                navigate('/login')
              }}
            >
              <IconLogout />
              Sair
            </button>
          </>
        ) : (
          <NavLink to="/login" className={linkClass}>
            <IconLogin />
            Entrar
          </NavLink>
        )}
      </div>
    </aside>
  )
}
