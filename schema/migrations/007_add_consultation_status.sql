ALTER TABLE tasks ADD COLUMN IF NOT EXISTS consultation_status VARCHAR(32) NULL CHECK (consultation_status IN ('pending', 'approved', 'rejected'));
