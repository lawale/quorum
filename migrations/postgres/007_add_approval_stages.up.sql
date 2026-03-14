-- Add stages JSONB column to policies
ALTER TABLE quorum.policies ADD COLUMN stages JSONB;

-- Migrate existing flat fields into a single-stage JSON array
UPDATE quorum.policies SET stages = jsonb_build_array(jsonb_build_object(
    'index', 0,
    'required_approvals', required_approvals,
    'allowed_checker_roles', COALESCE(allowed_checker_roles, 'null'::jsonb),
    'rejection_policy', rejection_policy,
    'max_checkers', max_checkers
));

-- Now make stages NOT NULL
ALTER TABLE quorum.policies ALTER COLUMN stages SET NOT NULL;

-- Drop the old flat columns
ALTER TABLE quorum.policies DROP COLUMN required_approvals;
ALTER TABLE quorum.policies DROP COLUMN allowed_checker_roles;
ALTER TABLE quorum.policies DROP COLUMN rejection_policy;
ALTER TABLE quorum.policies DROP COLUMN max_checkers;

-- Add stage_index to approvals (default 0 for existing rows)
ALTER TABLE quorum.approvals ADD COLUMN stage_index INT NOT NULL DEFAULT 0;

-- Replace unique constraint: now scoped per stage
ALTER TABLE quorum.approvals DROP CONSTRAINT approvals_request_id_checker_id_key;
ALTER TABLE quorum.approvals ADD CONSTRAINT approvals_request_id_checker_id_stage_idx_key
    UNIQUE (request_id, checker_id, stage_index);

-- Add current_stage to requests (default 0 for existing rows)
ALTER TABLE quorum.requests ADD COLUMN current_stage INT NOT NULL DEFAULT 0;
