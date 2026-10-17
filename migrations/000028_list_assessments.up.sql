CREATE TABLE list_assessment_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_item_id UUID NOT NULL REFERENCES list_items(id) ON DELETE CASCADE,
    revision INT NOT NULL CHECK (revision > 0),
    kind TEXT NOT NULL CHECK (kind IN ('multiple_choice', 'short_answer')),
    prompt TEXT NOT NULL,
    options JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(options) = 'array'),
    accepted_answers JSONB NOT NULL CHECK (jsonb_typeof(accepted_answers) = 'array'),
    case_sensitive BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (list_item_id, revision)
);

CREATE TABLE list_assessment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    revision_id UUID NOT NULL REFERENCES list_assessment_revisions(id),
    student_id UUID NOT NULL REFERENCES users(id),
    answer TEXT NOT NULL,
    accepted BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assessment_attempts_student ON list_assessment_attempts (student_id, created_at DESC, id DESC);
CREATE INDEX idx_assessment_attempts_completion ON list_assessment_attempts (revision_id, student_id, accepted);
