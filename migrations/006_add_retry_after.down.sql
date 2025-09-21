-- Remove retry_after column and related indexes
DROP INDEX IF EXISTS idx_messages_retry_after;
DROP INDEX IF EXISTS idx_messages_queue_poll;
ALTER TABLE messages DROP COLUMN IF EXISTS retry_after;
