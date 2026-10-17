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

function studentSummaryText(stats: StudentStats, assessments: StudentOverview['assessment_stats']): string {
  const parts = [
    `${stats.total_submissions} envios de código`,
    `${assessments.attempts} respostas em exercícios objetivos`,
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
  const [onlyDifficulties, setOnlyDifficulties] = useState(false)
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
        <h1>progresso</h1>
        <p className="muted">Todos os alunos que já interagiram com desafios ou listas.</p>
      </section>

      {loading ? <p className="status-message">Carregando progresso...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error && students.length === 0 ? (
        <p className="status-message">Nenhum aluno interagiu com desafios ou listas ainda.</p>
      ) : null}

      <label className="telemetry-chip">
        <input type="checkbox" checked={onlyDifficulties} onChange={(event) => setOnlyDifficulties(event.target.checked)} />
        Mostrar apenas alunos com dificuldades recorrentes
      </label>
      <p className="muted">Dificuldade recorrente: desafio ainda não resolvido com pelo menos 3 respostas incorretas, erros de execução ou limites de tempo excedidos. Envios pendentes não contam como erro. Exercícios objetivos entram no filtro após três respostas não aceitas na versão atual, sem acerto.</p>
      {onlyDifficulties && !loading && !error && !students.some((student) => student.stats.recurring_difficulties + student.assessment_stats.recurring_difficulties > 0) ? (
        <p className="status-message">Nenhum aluno com dificuldades recorrentes.</p>
      ) : null}
      <div className="student-roster">
        {students.filter((student) => !onlyDifficulties || student.stats.recurring_difficulties + student.assessment_stats.recurring_difficulties > 0).map((student) => (
          <article key={student.student_id} className="panel student-card">
            <h2 className="student-card__email">{student.email}</h2>
            <span className="telemetry-chip">
              <span className="telemetry-chip__item">{studentSummaryText(student.stats, student.assessment_stats)}</span>
            </span>

            <div className="student-card__section">
              <h3 className="section-title">Acompanhamento pedagógico</h3>
              <p>{student.stats.unresolved_challenges} desafios ainda não resolvidos · {student.stats.recurring_difficulties} com dificuldades recorrentes</p>
              <p className="muted">Histórico de erros: {student.stats.wrong_answer_count} respostas incorretas · {student.stats.runtime_error_count} erros de execução · {student.stats.timeout_count} limites de tempo excedidos</p>
              {student.challenges.filter((challenge) => challenge.accepted_count === 0 && challenge.wrong_answer_count + challenge.runtime_error_count + challenge.timeout_count >= 3).map((challenge) => (
                <p key={challenge.challenge_id}><Link to={`/challenges/${challenge.challenge_id}`}>{challenge.challenge_title}</Link> — {challenge.wrong_answer_count} respostas incorretas, {challenge.runtime_error_count} erros de execução, {challenge.timeout_count} limites de tempo excedidos</p>
              ))}
              <h3 className="section-title">Evolução semanal</h3>
              <p className="muted">Semanas com atividade, iniciadas na segunda-feira (UTC). Cada desafio conta como resolvido apenas na primeira aceitação.</p>
              {student.weekly_progress.length === 0 ? <p className="muted">Nenhum envio para acompanhar ainda.</p> : (
                <div className="datagrid">
                <table aria-label={`Evolução semanal de ${student.email}`}>
                  <thead><tr><th scope="col">Semana de</th><th scope="col">Tentativas</th><th scope="col">Novos desafios resolvidos</th></tr></thead>
                  <tbody>{student.weekly_progress.map((week) => (
                    <tr key={week.week_start}><th scope="row">{week.week_start.split('-').reverse().join('/')}</th><td>{week.attempts}</td><td>{week.newly_solved}</td></tr>
                  ))}</tbody>
                </table>
                </div>
              )}
            </div>

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
              <h3 className="section-title">Múltipla escolha e resposta curta</h3>
              <p>{student.assessment_stats.attempts} respostas enviadas · {student.assessment_stats.accepted_attempts} aceitas no histórico</p>
              <p className="muted">Versões atuais: {student.assessment_stats.items_solved} exercícios resolvidos · {student.assessment_stats.recurring_difficulties} com dificuldades recorrentes</p>
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
