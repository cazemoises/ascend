ALTER TABLE submissions ADD COLUMN http_requests JSONB;
ALTER TABLE submissions ADD CONSTRAINT http_requests_test_run CHECK (http_requests IS NULL OR (is_test_run AND language = 'fastapi' AND jsonb_typeof(http_requests) = 'array'));
ALTER TABLE challenges ADD COLUMN lab_owner_id UUID REFERENCES users(id);
ALTER TABLE challenges ADD COLUMN lab_list_id UUID REFERENCES problem_lists(id);
