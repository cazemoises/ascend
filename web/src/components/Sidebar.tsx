import { useEffect, useState } from 'react'
import { NavLink } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'
import { SoundToggle } from './SoundToggle'
import { ThemeToggle } from './ThemeToggle'

const isOnlyLists = import.meta.env.VITE_ONLY_LISTS_MODE === 'true'

function IconMenu() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <line x1="3" y1="6" x2="21" y2="6" />
      <line x1="3" y1="12" x2="21" y2="12" />
      <line x1="3" y1="18" x2="21" y2="18" />
    </svg>
  )
}

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

function IconStudents() {
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
  const [mobileOpen, setMobileOpen] = useState(false)
  const closeMobile = () => setMobileOpen(false)

  // Evita que o menu fique "preso" aberto se a janela for redimensionada
  // (ou o dispositivo girado) para acima do breakpoint mobile.
  useEffect(() => {
    const query = window.matchMedia('(min-width: 921px)')
    const handleChange = (e: MediaQueryListEvent) => {
      if (e.matches) setMobileOpen(false)
    }
    query.addEventListener('change', handleChange)
    return () => query.removeEventListener('change', handleChange)
  }, [])

  return (
    <>
      <div className="mobile-topbar">
        <button
          type="button"
          className="mobile-topbar__menu-btn"
          aria-label="Abrir menu de navegação"
          aria-expanded={mobileOpen}
          onClick={() => setMobileOpen(true)}
        >
          <IconMenu />
        </button>
        <span className="mobile-topbar__brand">
          <span className="sidebar__brand-mark">▲</span>
          Ascend
        </span>
      </div>

      {mobileOpen ? (
        <div className="sidebar-overlay" onClick={closeMobile} aria-hidden="true" />
      ) : null}

      <aside className={mobileOpen ? 'sidebar sidebar--open' : 'sidebar'}>
        <div className="sidebar__brand">
          <span className="sidebar__brand-mark">▲</span>
          <span>Ascend</span>
        </div>

        <nav className="sidebar__nav" aria-label="Navegação principal">
          {/* DESAFIOS — indisponível em modo só-listas */}
          {!isOnlyLists ? (
            <NavLink to="/" end className={linkClass} onClick={closeMobile}>
              <IconChallenges />
              Desafios
            </NavLink>
          ) : null}

          {/* LISTAS — sempre visível: navegação pública, não exige login */}
          <NavLink to="/listas" className={linkClass} onClick={closeMobile}>
            <IconLists />
            Listas
          </NavLink>

          {/* HISTÓRICO — indisponível em modo só-listas */}
          {isAuthenticated && !isOnlyLists ? (
            <NavLink to="/submissions" className={linkClass} onClick={closeMobile}>
              <IconHistory />
              Histórico
            </NavLink>
          ) : null}

          {/* PROGRESSO — visão do professor sobre todos os alunos */}
          {isTeacher ? (
            <NavLink to="/progresso" className={linkClass} onClick={closeMobile}>
              <IconStudents />
              Progresso
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

          <div className="sidebar__utility-row">
            <ThemeToggle />
            <SoundToggle />
          </div>
        </div>
      </aside>
    </>
  )
}