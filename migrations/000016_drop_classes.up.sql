DROP INDEX IF EXISTS idx_challenges_class;
ALTER TABLE challenges DROP COLUMN IF EXISTS class_id;
DROP INDEX IF EXISTS idx_class_enrollments_student;
DROP TABLE IF EXISTS class_enrollments;
DROP TABLE IF EXISTS classes;
