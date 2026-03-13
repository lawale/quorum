-- Add stages column to policies (NVARCHAR(MAX) for JSON)
ALTER TABLE policies ADD stages NVARCHAR(MAX);

-- Migrate existing flat fields into a single-stage JSON array
-- MSSQL doesn't have jsonb_build_object, so we construct the JSON string manually
UPDATE policies SET stages = '[{"index":0,"required_approvals":' + CAST(required_approvals AS NVARCHAR(10))
    + CASE WHEN allowed_checker_roles IS NOT NULL THEN ',"allowed_checker_roles":' + allowed_checker_roles ELSE '' END
    + ',"rejection_policy":"' + rejection_policy + '"'
    + CASE WHEN max_checkers IS NOT NULL THEN ',"max_checkers":' + CAST(max_checkers AS NVARCHAR(10)) ELSE '' END
    + '}]';

-- Make stages NOT NULL
ALTER TABLE policies ALTER COLUMN stages NVARCHAR(MAX) NOT NULL;

-- Drop flat columns
ALTER TABLE policies DROP COLUMN required_approvals;
ALTER TABLE policies DROP COLUMN allowed_checker_roles;
ALTER TABLE policies DROP COLUMN rejection_policy;
ALTER TABLE policies DROP COLUMN max_checkers;

-- Add stage_index to approvals
ALTER TABLE approvals ADD stage_index INT NOT NULL DEFAULT 0;

-- Replace unique constraint on approvals
DECLARE @constraintName NVARCHAR(200);
SELECT @constraintName = name FROM sys.key_constraints
WHERE parent_object_id = OBJECT_ID('approvals') AND type = 'UQ';
IF @constraintName IS NOT NULL
    EXEC('ALTER TABLE approvals DROP CONSTRAINT ' + @constraintName);

ALTER TABLE approvals ADD CONSTRAINT approvals_request_id_checker_id_stage_idx_key
    UNIQUE (request_id, checker_id, stage_index);

-- Add current_stage to requests
ALTER TABLE requests ADD current_stage INT NOT NULL DEFAULT 0;
