-- Migration: Add process_path and idle_time fields to activity_events
-- Date: 2025-10-29

-- Add new columns to existing table
ALTER TABLE monitoring.activity_events 
ADD COLUMN IF NOT EXISTS process_path String DEFAULT '' AFTER process_name;

ALTER TABLE monitoring.activity_events 
ADD COLUMN IF NOT EXISTS idle_time UInt32 DEFAULT 0 AFTER duration;
