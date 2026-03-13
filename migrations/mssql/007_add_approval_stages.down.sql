-- Re-add flat columns
ALTER TABLE policies ADD required_approvals INT NOT NULL DEFAULT 1;
ALTER TABLE policies ADD allowed_checker_roles NVARCHAR(MAX);
ALTER TABLE policies ADD rejection_policy NVARCHAR(50) NOT NULL DEFAULT 'any';
ALTER TABLE policies ADD max_checkers INT;

-- Restore flat fields from stage 0 using JSON_VALUE
UPDATE policies SET
    required_approvals = COALESCE(CAST(JSON_VALUE(stages, '$[0].required_approvals') AS INT), 1),
    allowed_checker_roles = JSON_QUERY(stages, '$[0].allowed_checker_roles'),
    rejection_policy = COALESCE(JSON_VALUE(stages, '$[0].rejection_policy'), 'any'),
    max_checkers = CAST(JSON_VALUE(stages, '$[0].max_checkers') AS INT);

-- Drop stages
ALTER TABLE policies DROP COLUMN stages;

-- Restore original unique constraint on approvals
DECLARE @constraintName NVARCHAR(200);
SELECT @constraintName = name FROM sys.key_constraints
WHERE parent_object_id = OBJECT_ID('approvals') AND type = 'UQ';
IF @constraintName IS NOT NULL
    EXEC('ALTER TABLE approvals DROP CONSTRAINT ' + @constraintName);

ALTER TABLE approvals ADD CONSTRAINT approvals_request_id_checker_id_key UNIQUE (request_id, checker_id);
ALTER TABLE approvals DROP COLUMN stage_index;

-- Drop current_stage from requests
ALTER TABLE requests DROP COLUMN current_stage;
