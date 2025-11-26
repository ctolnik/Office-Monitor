#!/bin/bash
# Проверка что API реально возвращает

echo "=== Checking API Response for 2025-11-25 ==="
echo ""

# Пробуем через curl (если доступен)
if command -v curl &> /dev/null; then
    echo "Testing with curl..."
    curl -s "http://monitor.net.gslaudit.ru/api/reports/daily/a-kiv?date=2025-11-25" | head -c 2000
    echo ""
else
    echo "curl not available"
fi

echo ""
echo "=== INSTRUCTIONS ==="
echo "Вам нужно вручную проверить что возвращает API:"
echo ""
echo "1. Откройте DevTools (F12)"
echo "2. Network tab"
echo "3. Обновите страницу отчёта"
echo "4. Найдите запрос: /api/reports/daily/a-kiv?date=2025-11-25"
echo "5. Response tab -> скопируйте ПЕРВЫЕ 100 СТРОК"
echo ""
echo "Если там 'applications': []  -> сервер НЕ ОБНОВЛЁН"
echo "Если там 'applications': [{...}, {...}]  -> проблема во FRONTEND"

