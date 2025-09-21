-- Add retry_after column for database-based retry mechanism
ALTER TABLE messages ADD COLUMN retry_after timestamptz;

-- Add index for efficient retry processing
CREATE INDEX idx_messages_retry_after ON messages (status, retry_after) 
WHERE status = 'FAILED_TEMP' AND retry_after IS NOT NULL;

-- Add index for efficient queue polling with express priority
CREATE INDEX idx_messages_queue_poll ON messages (status, express DESC, created_at ASC) 
WHERE status = 'QUEUED';
