ALTER TABLE submissions
  DROP COLUMN exec_time_ms,
  DROP COLUMN memory_peak_mb,
  DROP COLUMN stderr;
