import { useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(email, password)
      navigate(from, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Falha ao entrar')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="auth-split">
      <section className="auth-split__brand">
        <p className="auth-wordmark">
          ASCEND <span>//</span> CODE JUDGE PRO
        </p>

        <div>
          <h1 className="auth-headline">
            Escreva. Submeta. <em>Ascenda.</em>
          </h1>
          <p className="auth-tagline">
            Sua solução compila e roda em um sandbox isolado, contra todos os
            casos de teste, com veredito em segundos.
          </p>
          <ul className="auth-signals">
            <li>Execução isolada por submissão — Docker sandbox</li>
            <li>Python, Go e JavaScript com templates prontos</li>
            <li>Histórico completo de vereditos por desafio</li>
          </ul>
        </div>

        <p className="auth-footnote">Sandbox Docker · Fila Redis · Runtime Go</p>
      </section>

      <section className="auth-split__panel">
        <form className="auth-form" onSubmit={handleSubmit}>
          <p className="eyebrow">Bem-vindo de volta</p>
          <h1>Entrar</h1>
          <p className="auth-form__lede">Acesse sua conta para submeter soluções.</p>
          <label>
            E-mail
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              required
            />
          </label>
          <label>
            Senha
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </label>

          {error ? <p className="status-message status-error">{error}</p> : null}

          <button type="submit" disabled={submitting}>
            {submitting ? 'Entrando...' : 'Entrar'}
          </button>
          <p className="muted">
            Não tem conta? <Link to="/register">Cadastre-se</Link>
          </p>
        </form>
      </section>
    </main>
  )
}
