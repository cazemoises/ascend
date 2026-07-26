const VERDICT_LABELS: Record<string, string> = {
  accepted: 'Aceito',
  wrong_answer: 'Resposta errada',
  runtime_error: 'Erro de execução',
  time_limit_exceeded: 'Tempo excedido',
  pending: 'Na fila',
  running: 'Executando',
  error: 'Erro',
}

export function VerdictBadge({ status }: { status: string }) {
  return (
    <span className={`verdict verdict--${status}`}>
      {VERDICT_LABELS[status] ?? status}
    </span>
  )
}
