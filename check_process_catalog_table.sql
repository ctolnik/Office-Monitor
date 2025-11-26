-- Check if process_catalog table exists
SHOW TABLES FROM monitoring LIKE 'process_catalog';

-- Show structure if exists
DESC monitoring.process_catalog;

-- Count rows
SELECT count(*) FROM monitoring.process_catalog;
