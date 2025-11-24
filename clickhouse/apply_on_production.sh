#!/bin/bash
# Safe migration script for production ClickHouse
# This script only creates missing tables without touching existing indexes

echo "🚀 Applying activity_segments migration..."
echo ""

# Check if running in docker
if command -v docker &> /dev/null; then
    echo "✅ Docker found, using docker exec"
    docker exec -i clickhouse clickhouse-client --database=monitoring < add_activity_segments.sql
else
    echo "⚠️  Docker not found, using local clickhouse-client"
    clickhouse-client --database=monitoring < add_activity_segments.sql
fi

echo ""
echo "🎉 Migration complete!"
echo ""
echo "Checking created tables..."
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    name, 
    engine,
    create_table_query 
FROM system.tables 
WHERE database='monitoring' AND name IN ('activity_segments', 'daily_activity_summary', 'program_usage_daily')
FORMAT Vertical
"
