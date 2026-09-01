import { Outlet } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'
import { useSharedTheme } from '../lib/sharedTheme'
import { SoundToggle } from './SoundToggle'
import { useAscendShellNav } from './useAscendShellNav'

// Sidebar própria removida — navegação e identidade agora vivem só na
// sidebar do Portal (ver useAscendShellNav.ts e PORTAL_SHELL_PROTOCOL.md
// no repo do portal). "Ver como aluno" e o toggle de som continuam: são
// funcionalidade/preferência do app, não shell nem identidade de conta
// (mesma distinção já aplicada ao badge do Queryzão e à cor de acento do
// Yamabiko) — relocados pra um toolbar flutuante já que a sidebar que os
// hospedava não existe mais.
export function AppLayout() {
  const { isRealTeacher, viewingAsStudent, setViewingAsStudent } = useAuth()
  useAscendShellNav()
  useSharedTheme()

  return (
    <div className="app-shell">
      <div className="app-main">
        <Outlet />
      </div>
      <div className="app-toolbar">
        {isRealTeacher ? (
          <button
            type="button"
            className="app-toolbar__view-as"
            onClick={() => setViewingAsStudent(!viewingAsStudent)}
          >
            {viewingAsStudent ? 'Ver como professor' : 'Ver como aluno'}
          </button>
        ) : null}
        <SoundToggle />
      </div>
    </div>
  )
}
