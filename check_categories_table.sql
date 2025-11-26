-- Проверить есть ли таблица application_categories
SHOW TABLES FROM monitoring LIKE '%categor%';

-- Если есть, показать структуру
DESC monitoring.application_categories;

-- Показать первые 10 записей
SELECT * FROM monitoring.application_categories LIMIT 10;
