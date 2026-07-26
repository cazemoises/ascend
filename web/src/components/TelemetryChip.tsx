function IconClock() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <polyline points="12 6 12 12 16 14" />
    </svg>
  )
}

function IconMemory() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="4" y="4" width="16" height="16" rx="1" />
      <path d="M9 4v3M15 4v3M9 17v3M15 17v3M4 9h3M4 15h3M17 9h3M17 15h3" />
    </svg>
  )
}

// Fixed time/memory pair, always in this order and format, wherever a
// challenge's sandbox limits are surfaced — the feed card, the submission
// header, and the studio's live preview all render the same readout.
export function TelemetryChip({
  timeLimitMs,
  memoryLimitMb,
}: {
  timeLimitMs: number
  memoryLimitMb: number
}) {
  return (
    <span className="telemetry-chip">
      <span className="telemetry-chip__item">
        <IconClock />
        {timeLimitMs}ms
      </span>
      <span className="telemetry-chip__item">
        <IconMemory />
        {memoryLimitMb}MB
      </span>
    </span>
  )
}
