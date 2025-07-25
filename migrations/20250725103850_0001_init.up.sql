-- Use pgcrypto for better portability (e.g., on AWS RDS)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table for Jobs
CREATE TABLE jobs (
  uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  id TEXT NOT NULL,
  version INT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  valid_to TIMESTAMPTZ,
  status TEXT,
  rate NUMERIC(10, 2),
  title TEXT,
  company_id TEXT,
  contractor_id TEXT,
  UNIQUE(id, version)
);

-- Table for Timelogs
CREATE TABLE timelogs (
  uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  id TEXT NOT NULL,
  version INT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  valid_to TIMESTAMPTZ,
  duration BIGINT,
  time_start BIGINT,
  time_end BIGINT,
  type TEXT,
  job_uid UUID NOT NULL REFERENCES jobs(uid), -- Foreign Key to a specific job version
  UNIQUE(id, version)
);

-- Table for Payment Line Items
CREATE TABLE payment_line_items (
  uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  id TEXT NOT NULL,
  version INT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  valid_to TIMESTAMPTZ,
  job_uid UUID NOT NULL REFERENCES jobs(uid),         -- Foreign Key to a specific job version
  timelog_uid UUID NOT NULL REFERENCES timelogs(uid), -- Foreign Key to a specific timelog version
  amount NUMERIC(10, 2),
  status TEXT,
  UNIQUE(id, version)
);

-- === PERFORMANCE INDEXES ===

-- Partial indexes for querying the "latest" version of records quickly
CREATE INDEX idx_jobs_latest_company ON jobs(company_id) WHERE valid_to IS NULL;
CREATE INDEX idx_jobs_latest_contractor ON jobs(contractor_id) WHERE valid_to IS NULL;
CREATE INDEX idx_timelogs_latest_job ON timelogs(job_uid) WHERE valid_to IS NULL;
CREATE INDEX idx_lineitems_latest_job ON payment_line_items(job_uid) WHERE valid_to IS NULL;

-- Indexes for fetching the full history of a single entity quickly
CREATE INDEX idx_jobs_id ON jobs(id);
CREATE INDEX idx_timelogs_id ON timelogs(id);
CREATE INDEX idx_lineitems_id ON payment_line_items(id); 