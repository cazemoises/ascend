import { Outlet } from 'react-router-dom'

import { Sidebar } from './Sidebar'

export function AppLayout() {
  return (
    <div className="app-shell">
      <Sidebar />
      <div className="app-main">
        <Outlet />
      </div>
    </div>
  )
}
