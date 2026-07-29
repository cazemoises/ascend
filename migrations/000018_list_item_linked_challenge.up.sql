-- Links a list item to an auto-correctable challenge: when set, the item's
-- completion is derived from the student having solved that challenge
-- (accepted, non-test-run submission) instead of a self-declared checkbox.
-- ON DELETE SET NULL: deleting the challenge un-links the item rather than
-- blocking the delete or cascading it away.
ALTER TABLE list_items
  ADD COLUMN IF NOT EXISTS linked_challenge_id UUID NULL REFERENCES challenges(id) ON DELETE SET NULL;
