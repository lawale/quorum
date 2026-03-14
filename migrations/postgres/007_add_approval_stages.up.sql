-- Add stages JSONB column to policies
ALTER TABLE policies ADD COLUMN stages JSONB;

-- Migrate existing flat fields into a single-stage JSON array
UPDATE policies SET stages = jsonb_build_array(jsonb_build_object(
    'index', 0,
    'required_approvals', required_approvals,
    'allowed_checker_roles', COALESCE(allowed_checker_roles, 'null'::jsonb),
    'rejection_policy', rejection_policy,
    'max_checkers', max_checkers
));

-- Now make stages NOT NULL
ALTER TABLE policies ALTER COLUMN stages SET NOT NULL;

-- Drop the old flat columns
ALTER TABLE policies DROP COLUMN required_approvals;
ALTER TABLE policies DROP COLUMN allowed_checker_roles;
ALTER TABLE policies DROP COLUMN rejection_policy;
ALTER TABLE policies DROP COLUMN max_checkers;

-- Add stage_index to approvals (default 0 for existing rows)
ALTER TABLE approvals ADD COLUMN stage_index INT NOT NULL DEFAULT 0;

-- Replace unique constraint: now scoped per stage
ALTER TABLE approvals DROP CONSTRAINT approvals_request_id_checker_id_key;
ALTER TABLE approvals ADD CONSTRAINT approvals_request_id_checker_id_stage_idx_key
    UNIQUE (request_id, checker_id, stage_index);

-- Add current_stage to requests (default 0 for existing rows)
ALTER TABLE requests ADD COLUMN current_stage INT NOT NULL DEFAULT 0;
