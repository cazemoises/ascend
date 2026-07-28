import { NavLink } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

const isOnlyLists = import.meta.env.VITE_ONLY_LISTS_MODE === 'true'

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

const linkClass = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'sidebar__link sidebar__link--active' : 'sidebar__link'

export function Sidebar() {
  const { user, isAuthenticated, isTeacher, isRealTeacher, viewingAsStudent, setViewingAsStudent } =
    useAuth()

  return (
    <aside className="sidebar">
      <div className="sidebar__brand">
        <span className="sidebar__brand-mark">▲</span>
        <span>Ascend</span>
      </div>

      <nav className="sidebar__nav" aria-label="Navegação principal">
        {/* DESAFIOS — indisponível em modo só-listas */}
        {!isOnlyLists ? (
          <NavLink to="/" end className={linkClass}>
            <IconChallenges />
            Desafios
          </NavLink>
        ) : null}

        {/* LISTAS — sempre visível: navegação pública, não exige login */}
        <NavLink to="/listas" className={linkClass}>
          <IconLists />
          Listas
        </NavLink>

        {/* HISTÓRICO — indisponível em modo só-listas */}
        {isAuthenticated && !isOnlyLists ? (
          <NavLink to="/submissions" className={linkClass}>
            <IconHistory />
            Histórico
          </NavLink>
        ) : null}

        {/* TURMAS — indisponível em modo só-listas */}
        {isTeacher && !isOnlyLists ? (
          <NavLink to="/turmas" className={linkClass}>
            <IconClasses />
            Turmas
          </NavLink>
        ) : null}
      </nav>

      <div className="sidebar__footer">
        {isAuthenticated && user !== null ? (
          <div className="sidebar__user">
            <span className="sidebar__user-email" title={user.email}>
              {user.email}
            </span>
            <span
              className={
                isTeacher
                  ? 'sidebar__role sidebar__role--teacher'
                  : 'sidebar__role'
              }
            >
              {isTeacher ? 'Professor' : 'Estudante'}
            </span>
          </div>
        ) : (
          <span className="sidebar__user-email">Não identificado</span>
        )}

        {/* Só um teacher de verdade pode alternar — checa isRealTeacher, não
            isTeacher, senão o toggle desapareceria assim que ativado. */}
        {isRealTeacher ? (
          <button
            type="button"
            className="sidebar__view-as-toggle"
            onClick={() => setViewingAsStudent(!viewingAsStudent)}
          >
            {viewingAsStudent ? 'Ver como professor' : 'Ver como aluno'}
          </button>
        ) : null}
      </div>
    </aside>
  )
}