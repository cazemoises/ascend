function IconCalendar() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="4" width="18" height="18" rx="2" />
      <line x1="16" y1="2" x2="16" y2="6" />
      <line x1="8" y1="2" x2="8" y2="6" />
      <line x1="3" y1="10" x2="21" y2="10" />
    </svg>
  )
}

const MONTH_ABBR_PT = [
  'JAN', 'FEV', 'MAR', 'ABR', 'MAI', 'JUN',
  'JUL', 'AGO', 'SET', 'OUT', 'NOV', 'DEZ',
]

// week_start/week_end are UTC-midnight timestamps — read with the UTC
// getters so the displayed day never shifts with the viewer's timezone.
function formatWeekRange(weekStart: string, weekEnd: string): string {
  const start = new Date(weekStart)
  const end = new Date(weekEnd)
  const startDay = start.getUTCDate()
  const endDay = end.getUTCDate()
  const endMonth = MONTH_ABBR_PT[end.getUTCMonth()]

  if (start.getUTCFullYear() === end.getUTCFullYear() && start.getUTCMonth() === end.getUTCMonth()) {
    return `${startDay}–${endDay} ${endMonth}`
  }
  const startMonth = MONTH_ABBR_PT[start.getUTCMonth()]
  return `${startDay} ${startMonth} – ${endDay} ${endMonth}`
}

// Same visual pattern as TelemetryChip (mono, muted, icon + text) — used
// wherever a list's week range is surfaced: the feed card and the detail
// header.
export function WeekChip({ weekStart, weekEnd }: { weekStart: string; weekEnd: string }) {
  return (
    <span className="telemetry-chip">
      <span className="telemetry-chip__item">
        <IconCalendar />
        {formatWeekRange(weekStart, weekEnd)}
      </span>
    </span>
  )
}
