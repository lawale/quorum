-- Re-add flat columns to policies
ALTER TABLE policies ADD COLUMN required_approvals INT NOT NULL DEFAULT 1;
ALTER TABLE policies ADD COLUMN allowed_checker_roles JSONB;
ALTER TABLE policies ADD COLUMN rejection_policy VARCHAR(50) NOT NULL DEFAULT 'any';
ALTER TABLE policies ADD COLUMN max_checkers INT;

-- Restore flat fields from stage 0 in the stages array
UPDATE policies SET
    required_approvals = COALESCE((stages->0->>'required_approvals')::INT, 1),
    allowed_checker_roles = CASE
        WHEN stages->0->'allowed_checker_roles' = 'null'::jsonb THEN NULL
        ELSE stages->0->'allowed_checker_roles'
    END,
    rejection_policy = COALESCE(stages->0->>'rejection_policy', 'any'),
    max_checkers = (stages->0->>'max_checkers')::INT;

-- Drop stages column
ALTER TABLE policies DROP COLUMN stages;

-- Restore original unique constraint on approvals
ALTER TABLE approvals DROP CONSTRAINT approvals_request_id_checker_id_stage_idx_key;
ALTER TABLE approvals ADD CONSTRAINT approvals_request_id_checker_id_key UNIQUE (request_id, checker_id);
ALTER TABLE approvals DROP COLUMN stage_index;

-- Drop current_stage from requests
ALTER TABLE requests DROP COLUMN current_stage;
