// Rotated bicolor diamond: warm accent on top, teal on bottom — same
// pairing as the rest of the system (primary/CTA vs. navigation/progress).
// Replaces the interim "▲" placeholder glyph.
export function BrandMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" className="brand-mark">
      <path d="M12 2 L22 12 L2 12 Z" fill="var(--accent-terracota)" />
      <path d="M2 12 L22 12 L12 22 Z" fill="var(--accent-teal)" />
    </svg>
  )
}
