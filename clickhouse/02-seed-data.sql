-- ============================================================================
-- Office Monitor - Seed Data
-- ============================================================================
-- This migration populates initial data for application categories
-- Safe to run multiple times (uses INSERT...ON CONFLICT DO NOTHING)
-- ============================================================================

\echo '========================================='
\echo 'Starting seed data migration...'
\echo '========================================='

\echo 'Populating application_categories table...'

-- Clear existing seed data (optional - comment out if you want to keep existing data)
-- DELETE FROM monitoring.application_categories WHERE created_by = 'system';

INSERT INTO monitoring.application_categories 
(process_name, process_pattern, category, created_by, updated_by, is_active) 
VALUES

-- ============================================================================
-- PRODUCTIVE APPLICATIONS
-- ============================================================================

-- IDE and Code Editors
('code.exe', 'code*', 'productive', 'system', 'system', 1),
('idea64.exe', 'idea*', 'productive', 'system', 'system', 1),
('pycharm64.exe', 'pycharm*', 'productive', 'system', 'system', 1),
('goland64.exe', 'goland*', 'productive', 'system', 'system', 1),
('webstorm64.exe', 'webstorm*', 'productive', 'system', 'system', 1),
('notepad++.exe', 'notepad++*', 'productive', 'system', 'system', 1),
('sublime_text.exe', 'sublime*', 'productive', 'system', 'system', 1),
('vim.exe', '', 'productive', 'system', 'system', 1),
('nvim.exe', '', 'productive', 'system', 'system', 1),

-- Terminals
('powershell.exe', 'powershell*', 'productive', 'system', 'system', 1),
('cmd.exe', '', 'productive', 'system', 'system', 1),
('terminal.exe', 'terminal*', 'productive', 'system', 'system', 1),
('wt.exe', '', 'productive', 'system', 'system', 1),
('bash.exe', '', 'productive', 'system', 'system', 1),

-- Database Tools
('datagrip64.exe', 'datagrip*', 'productive', 'system', 'system', 1),
('ssms.exe', '', 'productive', 'system', 'system', 1),
('mysqld.exe', '', 'productive', 'system', 'system', 1),
('postgres.exe', '', 'productive', 'system', 'system', 1),

-- Microsoft Office
('excel.exe', '', 'productive', 'system', 'system', 1),
('winword.exe', '', 'productive', 'system', 'system', 1),
('powerpnt.exe', '', 'productive', 'system', 'system', 1),
('outlook.exe', '', 'productive', 'system', 'system', 1),
('onenote.exe', '', 'productive', 'system', 'system', 1),

-- 1C Enterprise
('1cv8.exe', '1cv8*', 'productive', 'system', 'system', 1),
('1cv8c.exe', '1cv8c*', 'productive', 'system', 'system', 1),

-- Git Clients
('git.exe', 'git*', 'productive', 'system', 'system', 1),
('gitkraken.exe', 'gitkraken*', 'productive', 'system', 'system', 1),
('sourcetree.exe', 'sourcetree*', 'productive', 'system', 'system', 1),

-- Docker and DevOps
('docker.exe', 'docker*', 'productive', 'system', 'system', 1),
('kubectl.exe', 'kubectl*', 'productive', 'system', 'system', 1),

-- Design Tools
('photoshop.exe', 'photoshop*', 'productive', 'system', 'system', 1),
('illustrator.exe', 'illustrator*', 'productive', 'system', 'system', 1),
('figma.exe', 'figma*', 'productive', 'system', 'system', 1),

-- ============================================================================
-- COMMUNICATION APPLICATIONS
-- ============================================================================

('teams.exe', 'teams*', 'communication', 'system', 'system', 1),
('slack.exe', 'slack*', 'communication', 'system', 'system', 1),
('telegram.exe', 'telegram*', 'communication', 'system', 'system', 1),
('skype.exe', 'skype*', 'communication', 'system', 'system', 1),
('zoom.exe', 'zoom*', 'communication', 'system', 'system', 1),
('discord.exe', 'discord*', 'communication', 'system', 'system', 1),
('thunderbird.exe', 'thunderbird*', 'communication', 'system', 'system', 1),

-- ============================================================================
-- NEUTRAL APPLICATIONS (Browsers - depends on usage)
-- ============================================================================

('chrome.exe', 'chrome*', 'neutral', 'system', 'system', 1),
('firefox.exe', 'firefox*', 'neutral', 'system', 'system', 1),
('msedge.exe', 'msedge*', 'neutral', 'system', 'system', 1),
('opera.exe', 'opera*', 'neutral', 'system', 'system', 1),
('brave.exe', 'brave*', 'neutral', 'system', 'system', 1),
('iexplore.exe', '', 'neutral', 'system', 'system', 1),

-- File Managers
('explorer.exe', 'explorer*', 'neutral', 'system', 'system', 1),
('totalcmd64.exe', 'totalcmd*', 'neutral', 'system', 'system', 1),

-- ============================================================================
-- UNPRODUCTIVE APPLICATIONS
-- ============================================================================

-- Games
('steam.exe', 'steam*', 'unproductive', 'system', 'system', 1),
('epicgameslauncher.exe', 'epicgames*', 'unproductive', 'system', 'system', 1),
('gog.exe', 'gog*', 'unproductive', 'system', 'system', 1),

-- Entertainment
('spotify.exe', 'spotify*', 'unproductive', 'system', 'system', 1),
('vlc.exe', 'vlc*', 'unproductive', 'system', 'system', 1),

-- Social Media (URL patterns)
('youtube.com', '.*youtube.*', 'unproductive', 'system', 'system', 1),
('facebook.com', '.*facebook.*', 'unproductive', 'system', 'system', 1),
('vk.com', '.*vk\\.com.*', 'unproductive', 'system', 'system', 1),
('instagram.com', '.*instagram.*', 'unproductive', 'system', 'system', 1),
('twitter.com', '.*twitter.*', 'unproductive', 'system', 'system', 1),
('reddit.com', '.*reddit.*', 'unproductive', 'system', 'system', 1),
('tiktok.com', '.*tiktok.*', 'unproductive', 'system', 'system', 1),

-- ============================================================================
-- ENTERTAINMENT APPLICATIONS
-- ============================================================================

('netflix.exe', 'netflix*', 'entertainment', 'system', 'system', 1),
('kodi.exe', 'kodi*', 'entertainment', 'system', 'system', 1),
('plex.exe', 'plex*', 'entertainment', 'system', 'system', 1);

\echo '========================================='
\echo 'Seed data migration completed!'
\echo ''
SELECT 'Total categories: ' || toString(count(*)) as result 
FROM monitoring.application_categories;
\echo ''
SELECT 'By category:' as result;
SELECT 
    category,
    count(*) as count
FROM monitoring.application_categories 
GROUP BY category 
ORDER BY category;
\echo '========================================='
