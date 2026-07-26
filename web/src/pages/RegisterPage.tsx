import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

export function RegisterPage() {
  const { register } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (password.length < 8) {
      setError('A senha deve ter no mínimo 8 caracteres')
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      await register(email, password)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Falha ao cadastrar')
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
            Comece a subir <em>hoje.</em>
          </h1>
          <p className="auth-tagline">
            Crie sua conta e submeta soluções que rodam em um sandbox isolado,
            contra todos os casos de teste, com veredito em segundos.
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
          <p className="eyebrow">Primeiro acesso</p>
          <h1>Criar conta</h1>
          <p className="auth-form__lede">Cadastre-se para submeter soluções aos desafios.</p>
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
              autoComplete="new-password"
              minLength={8}
              required
            />
          </label>

          {error ? <p className="status-message status-error">{error}</p> : null}

          <button type="submit" disabled={submitting}>
            {submitting ? 'Cadastrando...' : 'Cadastrar'}
          </button>
          <p className="muted">
            Já tem conta? <Link to="/login">Entrar</Link>
          </p>
        </form>
      </section>
    </main>
  )
}
