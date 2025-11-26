# 📦 ПОШАГОВАЯ УСТАНОВКА ИСПРАВЛЕННОЙ ВЕРСИИ

**Проблема:** На production запущена СТАРАЯ версия сервера  
**Решение:** Установить НОВУЮ версию из Replit

---

## ✅ ШАГ ЗА ШАГОМ:

### Шаг 1: Скопировать файл с Replit на production

**На вашем ЛОКАЛЬНОМ компьютере** (или откуда у вас SSH доступ):

```bash
# Скачать файл из Replit
# (Если Replit - это облачная среда, сначала скачайте server/server к себе)

# Затем скопировать на production:
scp server/server user@monitor.net.gslaudit.ru:/tmp/monitoring-server-new
```

**Важно:** Имя файла `server/server` (43MB) из этого Replit проекта

---

### Шаг 2: Подключиться к production серверу

```bash
ssh user@monitor.net.gslaudit.ru
```

---

### Шаг 3: Проверить что файл скопировался

```bash
ls -lh /tmp/monitoring-server-new

# Должно показать:
# -rw-r--r-- 1 user user 43M Nov 26 03:00 /tmp/monitoring-server-new
```

Если файла нет - вернитесь к Шагу 1!

---

### Шаг 4: Остановить сервер

```bash
sudo systemctl stop monitoring-server

# Проверить что остановился:
sudo systemctl status monitoring-server
# Должно быть: "inactive (dead)"
```

---

### Шаг 5: Заменить binary

```bash
sudo cp /tmp/monitoring-server-new /usr/local/bin/monitoring-server

# Установить права на выполнение:
sudo chmod +x /usr/local/bin/monitoring-server

# Проверить размер и дату:
ls -lh /usr/local/bin/monitoring-server

# Должно быть:
# -rwxr-xr-x 1 root root 43M Nov 26 03:XX monitoring-server
```

---

### Шаг 6: Запустить сервер

```bash
sudo systemctl start monitoring-server

# Проверить что запустился:
sudo systemctl status monitoring-server

# Должно быть: "active (running)"
```

---

### Шаг 7: Проверить логи - ИЩЕМ НОВЫЕ СТРОКИ!

```bash
docker logs monitoring-server --tail 50 | grep -E "(states|GetApplication)"
```

**Должны увидеть НОВЫЕ строки:**
```json
{"msg":"GetActivitySegmentsByUsername result","states":{"active":5,"idle":73}}
{"msg":"GetApplicationUsageFromSegments","query":"SELECT ... WHERE ... GROUP BY ..."}
```

Если этих строк НЕТ - значит старая версия всё ещё запущена!

---

### Шаг 8: Открыть отчёт в браузере

```
http://monitor.net.gslaudit.ru/reports/daily?username=a-kiv&date=2025-11-25
```

Обновить страницу (Ctrl+F5)

**Должны увидеть:**
- ✅ Приложения ЗАПОЛНЕНЫ (chrome.exe, notepad.exe и т.д.)
- ✅ Активное время НЕ 00:00:00
- ✅ Диаграммы заполнены

---

## 🔍 ЕСЛИ НЕ РАБОТАЕТ:

### Проверка 1: Правильный ли файл скопирован?

```bash
md5sum /usr/local/bin/monitoring-server
# Сравните с MD5 из Replit (см. ниже)
```

### Проверка 2: Запущен ли systemd сервис?

```bash
ps aux | grep monitoring-server
# Должен быть процесс /usr/local/bin/monitoring-server
```

### Проверка 3: Логи systemd

```bash
sudo journalctl -u monitoring-server --since "5 minutes ago"
# Ищите ошибки запуска
```

---

## 📋 MD5 правильного файла:

MD5 файла `server/server` из Replit (для проверки):
```
Будет показан после сборки
```

