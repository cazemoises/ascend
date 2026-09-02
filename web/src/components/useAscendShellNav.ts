import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

// Ponte com a sidebar do Portal — ver PORTAL_SHELL_PROTOCOL.md no repo do
// portal pro contrato completo. Substitui a navegação própria do Sidebar
// (removido): o Ascend não desenha mais sua sidebar/mobile-topbar, só
// informa o Portal do que navegar e reage quando o Portal manda navegar.
const NAV_CHANNEL = 'portal-shell'

// Origin confiável do bridge: SEMPRE a própria origin do documento — nunca
// uma allowlist de subdomínios standalone. Esses domínios nunca aparecem
// em event.origin quando o app está embutido no Portal: o Caddy serve
// Portal + apps sob a MESMA origin via path (/ascend/), nunca via
// subdomínio, nesse caso. Ver a seção "Validação de origin" do protocolo.
const TRUSTED_ORIGIN = window.location.origin

const isOnlyLists = import.meta.env.VITE_ONLY_LISTS_MODE === 'true'

const ICON_CHALLENGES =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>'
const ICON_LISTS =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>'
const ICON_HISTORY =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'
const ICON_STUDENTS =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>'

interface NavItemDef {
  id: string
  label: string
  path: string
  icon: string
}

// Mesmo conjunto condicional que o Sidebar.tsx removido já tinha: Desafios
// some no modo só-listas, Histórico exige sessão, Progresso exige papel de
// professor EFETIVO (respeita "Ver como aluno" — usa isTeacher, não
// isRealTeacher, mesma regra de acesso de antes).
function buildNavItems(isAuthenticated: boolean, isTeacher: boolean): NavItemDef[] {
  const items: NavItemDef[] = []
  if (!isOnlyLists) items.push({ id: 'challenges', label: 'desafios', path: '/', icon: ICON_CHALLENGES })
  items.push({ id: 'lists', label: 'listas', path: '/listas', icon: ICON_LISTS })
  if (isAuthenticated && !isOnlyLists) items.push({ id: 'history', label: 'histórico', path: '/submissions', icon: ICON_HISTORY })
  if (isTeacher) items.push({ id: 'progress', label: 'progresso', path: '/progresso', icon: ICON_STUDENTS })
  return items
}

function activeIdForPath(pathname: string, items: NavItemDef[]): string | null {
  const challengesItem = items.find((item) => item.path === '/')
  if (pathname === '/') return challengesItem?.id ?? null
  // do mais específico pro menos — evita "/" bater com tudo num
  // startsWith ingênuo (nenhum item hoje compartilha prefixo, mas a mesma
  // lógica do Yamabiko se aplica caso itens futuros passem a compartilhar).
  const match = [...items].reverse().find((item) => item.path !== '/' && pathname.startsWith(item.path))
  return match?.id ?? null
}

export function useAscendShellNav(): void {
  const { isAuthenticated, isTeacher } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    if (window.parent === window) return // não está embutido, ninguém pra avisar

    const items = buildNavItems(isAuthenticated, isTeacher)
    window.parent.postMessage(
      {
        channel: NAV_CHANNEL,
        type: 'nav:update',
        items: items.map(({ id, label, icon }) => ({ id, label, icon })),
        activeId: activeIdForPath(location.pathname, items),
      },
      TRUSTED_ORIGIN,
    )
  }, [location.pathname, isAuthenticated, isTeacher])

  useEffect(() => {
    function handleMessage(event: MessageEvent): void {
      if (event.origin !== TRUSTED_ORIGIN) return
      if (event.source !== window.parent) return
      const data = event.data as { channel?: string; type?: string; id?: string } | null
      if (!data || data.channel !== NAV_CHANNEL || data.type !== 'nav:go') return
      const item = buildNavItems(isAuthenticated, isTeacher).find((navItem) => navItem.id === data.id)
      if (item) navigate(item.path)
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [navigate, isAuthenticated, isTeacher])
}
