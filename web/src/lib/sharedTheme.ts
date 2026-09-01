import { useEffect } from 'react'

const STORAGE_KEY = 'theme'

// index.html já faz o stamp inicial de data-theme lendo a mesma chave de
// localStorage (evita flash) — este hook só cobre mudança AO VIVO feita
// pelo Portal (ou outra aba/iframe de mesma origin) enquanto o Ascend já
// está aberto. Sem toggle próprio aqui: o app só reage ao que o Portal
// decidir (ver PORTAL_SHELL_PROTOCOL.md no repo do portal).
export function useSharedTheme(): void {
  useEffect(() => {
    function handleStorage(e: StorageEvent): void {
      if (e.key !== STORAGE_KEY) return
      if (e.newValue === 'light') {
        document.documentElement.setAttribute('data-theme', 'light')
      } else {
        document.documentElement.removeAttribute('data-theme')
      }
    }
    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [])
}
