ALTER TABLE messages ADD COLUMN express boolean NOT NULL DEFAULT false;

-- Add index for express messages (for worker prioritization)
CREATE INDEX idx_messages_express ON messages (express, status) WHERE express = true;
