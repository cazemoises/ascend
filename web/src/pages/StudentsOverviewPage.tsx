import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import {
  getStudentsOverview,
  type StudentChallengeSummary,
  type StudentOverview,
  type StudentStats,
  type SubmissionAttempt,
} from '../api'
import { VerdictBadge } from '../components/VerdictBadge'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('pt-BR')
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('pt-BR')
}

function plural(count: number, singular: string, pluralForm: string): string {
  return count === 1 ? singular : pluralForm
}

function studentSummaryText(stats: StudentStats): string {
  const parts = [
    `${stats.total_submissions} ${plural(stats.total_submissions, 'tentativa', 'tentativas')}`,
    `${stats.challenges_accepted} ${plural(stats.challenges_accepted, 'desafio resolvido', 'desafios resolvidos')} de ${stats.challenges_attempted} ${plural(stats.challenges_attempted, 'tentado', 'tentados')}`,
  ]
  if (stats.avg_attempts_to_solve !== null) {
    parts.push(`média ${stats.avg_attempts_to_solve.toFixed(1)} tentativas/desafio`)
  }
  return parts.join(' · ')
}

function latestVerdict(attempts: SubmissionAttempt[]): string {
  return attempts.length > 0 ? attempts[attempts.length - 1].verdict : 'pending'
}

function ChallengeAccordion({ challenge }: { challenge: StudentChallengeSummary }) {
  const [isOpen, setIsOpen] = useState(false)

  return (
    <div className="challenge-accordion">
      <button
        type="button"
        className="audit-toggle challenge-accordion__toggle"
        aria-expanded={isOpen}
        onClick={() => setIsOpen((prev) => !prev)}
      >
        <span className={isOpen ? 'audit-caret audit-caret--open' : 'audit-caret'} aria-hidden="true">
          ▸
        </span>
        <span className="challenge-accordion__title">{challenge.challenge_title}</span>
        <span className="telemetry-chip">
          <span className="telemetry-chip__item">
            {challenge.total_attempts} {plural(challenge.total_attempts, 'tentativa', 'tentativas')} ·{' '}
            {challenge.accepted_count} {plural(challenge.accepted_count, 'aceita', 'aceitas')}
          </span>
        </span>
        <VerdictBadge status={latestVerdict(challenge.attempts)} />
      </button>

      {isOpen ? (
        <div className="challenge-accordion__attempts">
          {challenge.attempts.map((attempt) => (
            <Link
              key={attempt.submission_id}
              to={`/challenges/${challenge.challenge_id}/submissions/${attempt.submission_id}`}
              className="attempt-row"
            >
              <VerdictBadge status={attempt.verdict} />
              <span className="muted">{formatDateTime(attempt.created_at)}</span>
              <span className="attempt-row__language">{attempt.language}</span>
              <span className="muted">
                {attempt.exec_time_ms !== null ? `${attempt.exec_time_ms}ms` : '—'}
              </span>
            </Link>
          ))}
        </div>
      ) : null}
    </div>
  )
}

export function StudentsOverviewPage() {
  const [students, setStudents] = useState<StudentOverview[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    getStudentsOverview()
      .then((data) => {
        if (active) {
          setStudents(data)
        }
      })
      .catch((err) => {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar o progresso dos alunos')
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Área do professor</p>
        <h1>Progresso</h1>
        <p className="muted">Todos os alunos que já interagiram com desafios ou listas.</p>
      </section>

      {loading ? <p className="status-message">Carregando progresso...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error && students.length === 0 ? (
        <p className="status-message">Nenhum aluno interagiu com desafios ou listas ainda.</p>
      ) : null}

      <div className="student-roster">
        {students.map((student) => (
          <article key={student.student_id} className="panel student-card">
            <h2 className="student-card__email">{student.email}</h2>
            <span className="telemetry-chip">
              <span className="telemetry-chip__item">{studentSummaryText(student.stats)}</span>
            </span>

            <div className="student-card__section">
              <h3 className="section-title">Desafios</h3>
              {student.challenges.length === 0 ? (
                <p className="muted student-card__empty">Nenhum desafio enviado ainda.</p>
              ) : (
                <div className="challenge-accordion-list">
                  {student.challenges.map((challenge) => (
                    <ChallengeAccordion key={challenge.challenge_id} challenge={challenge} />
                  ))}
                </div>
              )}
            </div>

            <div className="student-card__section">
              <h3 className="section-title">Listas</h3>
              {student.list_completions.length === 0 ? (
                <p className="muted student-card__empty">Nenhum item concluído ainda.</p>
              ) : (
                <ul className="student-card__list">
                  {student.list_completions.map((completion) => (
                    <li
                      key={`${completion.list_title}-${completion.item_title}-${completion.completed_at}`}
                      className="student-card__row"
                    >
                      <span>
                        {completion.list_title} — {completion.item_title}
                      </span>
                      <span className="muted">{formatDate(completion.completed_at)}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </article>
        ))}
      </div>
    </main>
  )
}
