CREATE TABLE IF NOT EXISTS quorum.approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES quorum.requests(id) ON DELETE CASCADE,
    checker_id VARCHAR(255) NOT NULL,
    decision VARCHAR(50) NOT NULL,
    comment TEXT,
    stage_index INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT approvals_request_id_checker_id_stage_idx_key UNIQUE (request_id, checker_id, stage_index)
);

CREATE INDEX idx_approvals_request_id ON quorum.approvals (request_id);
