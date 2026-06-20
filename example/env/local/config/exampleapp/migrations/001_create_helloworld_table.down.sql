-- Drop trigger
DROP TRIGGER IF EXISTS update_helloworld_messages_updated_at ON helloworld_messages;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop index
DROP INDEX IF EXISTS idx_helloworld_messages_created_at;

-- Drop table
DROP TABLE IF EXISTS helloworld_messages;
