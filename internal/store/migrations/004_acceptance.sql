-- +goose Up
-- Acceptance criteria can be edited after a review: removed criteria are
-- hidden (deleted_at) instead of deleted, and every review item keeps the
-- criterion text it was judged against.
ALTER TABLE acceptance_criteria ADD COLUMN deleted_at TEXT;
ALTER TABLE acceptance_review_items ADD COLUMN description TEXT NOT NULL DEFAULT '';
CREATE INDEX ix_acceptance_criteria_issue ON acceptance_criteria(issue_id, position);
CREATE INDEX ix_acceptance_reviews_issue ON acceptance_reviews(issue_id, round);

-- +goose Down
DROP INDEX ix_acceptance_reviews_issue;
DROP INDEX ix_acceptance_criteria_issue;
ALTER TABLE acceptance_review_items DROP COLUMN description;
ALTER TABLE acceptance_criteria DROP COLUMN deleted_at;
