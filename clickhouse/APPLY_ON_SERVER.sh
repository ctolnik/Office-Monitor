#!/bin/bash
# Применение миграций на production сервере

echo "========================================="
echo "Applying ClickHouse Migrations"
echo "========================================="

# Применить схему
echo "1. Applying schema..."
cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring

if [ $? -eq 0 ]; then
    echo "✅ Schema applied successfully"
else
    echo "❌ Schema migration failed!"
    exit 1
fi

# Применить данные
echo ""
echo "2. Applying seed data..."
cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring

if [ $? -eq 0 ]; then
    echo "✅ Seed data applied successfully"
else
    echo "❌ Seed data migration failed!"
    exit 1
fi

# Проверить результат
echo ""
echo "3. Verifying..."
COUNT=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring -q "SELECT count(*) FROM application_categories" 2>&1)

if [[ "$COUNT" =~ ^[0-9]+$ ]] && [ "$COUNT" -gt 50 ]; then
    echo "✅ SUCCESS! Found $COUNT categories"
    
    echo ""
    echo "4. Restarting server to apply changes..."
    docker-compose restart server
    
    echo ""
    echo "========================================="
    echo "✅ ALL DONE!"
    echo "========================================="
    echo "Проверьте сайт - справочник программ должен работать!"
else
    echo "❌ FAILED! Categories count: $COUNT"
    echo ""
    echo "Checking for errors..."
    docker logs monitoring-clickhouse --tail 20
    exit 1
fi
