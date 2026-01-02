let selectedEmployee = null;
let employees = [];
let activities = [];

const categoryColors = {
    productive: '#4CAF50',
    unproductive: '#F44336',
    neutral: '#9E9E9E',
    communication: '#2196F3',
    system: '#607D8B',
    entertainment: '#FF9800'
};

const categoryNames = {
    productive: 'Продуктивно',
    unproductive: 'Непродуктивно',
    neutral: 'Нейтрально',
    communication: 'Коммуникация',
    system: 'Системные',
    entertainment: 'Развлечения'
};

function formatTime(date) {
    return new Date(date).toLocaleString('ru-RU');
}

function formatTimeShort(date) {
    return new Date(date).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
}

function formatDuration(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    
    if (hours > 0) {
        return `${hours}ч ${minutes}м`;
    } else if (minutes > 0) {
        return `${minutes}м ${secs}с`;
    } else {
        return `${secs}с`;
    }
}

async function fetchEmployees() {
    try {
        const response = await fetch('/api/employees');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        employees = data || [];
        renderEmployeeList();
        updateStats();
    } catch (error) {
        console.error('Error fetching employees:', error);
        employees = [];
    }
}

async function fetchRecentActivity() {
    try {
        const response = await fetch('/api/activity/recent');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        activities = data || [];
        renderRecentActivity();
    } catch (error) {
        console.error('Error fetching activity:', error);
        activities = [];
    }
}

function renderEmployeeList() {
    const container = document.getElementById('employee-list');
    const searchTerm = document.getElementById('search-employee').value.toLowerCase();
    const statusFilter = document.getElementById('status-filter').value;
    
    let filteredEmployees = employees;
    
    if (searchTerm) {
        filteredEmployees = filteredEmployees.filter(emp => 
            emp.username.toLowerCase().includes(searchTerm) ||
            emp.computer_name.toLowerCase().includes(searchTerm)
        );
    }
    
    if (statusFilter !== 'all') {
        filteredEmployees = filteredEmployees.filter(emp => emp.status === statusFilter);
    }
    
    container.innerHTML = filteredEmployees.map(emp => `
        <div class="employee-item ${emp.status} ${selectedEmployee === emp.username ? 'selected' : ''}"
             onclick="selectEmployee('${emp.username}')">
            <div class="employee-name">${emp.username}</div>
            <div class="employee-computer">${emp.computer_name}</div>
            <span class="employee-status status-${emp.status}">
                ${emp.status === 'active' ? 'Активен' : emp.status === 'idle' ? 'Неактивен' : 'Оффлайн'}
            </span>
        </div>
    `).join('');
}

function renderRecentActivity() {
    const container = document.getElementById('recent-activity');
    
    if (!activities || activities.length === 0) {
        container.innerHTML = '<p style="color: #999; text-align: center; padding: 20px;">Нет данных об активности</p>';
        return;
    }
    
    container.innerHTML = activities.slice(0, 20).map(activity => `
        <div class="activity-item">
            <div class="activity-header">
                <span class="activity-user">${activity.username}</span>
                <span class="activity-time">${formatTime(activity.timestamp)}</span>
            </div>
            <div class="activity-details">${activity.window_title || 'Без названия'}</div>
            <div class="activity-process">
                <span class="category-badge" style="background: ${categoryColors[activity.category] || categoryColors.neutral}">
                    ${categoryNames[activity.category] || 'Нейтрально'}
                </span>
                ${activity.process_name} (${formatDuration(activity.duration)})
            </div>
        </div>
    `).join('');
}

function updateStats() {
    const total = employees.length;
    const active = employees.filter(e => e.status === 'active').length;
    const idle = employees.filter(e => e.status === 'idle').length;
    const offline = employees.filter(e => e.status === 'offline').length;
    
    document.getElementById('total-employees').textContent = total;
    document.getElementById('active-employees').textContent = active;
    document.getElementById('idle-employees').textContent = idle;
    document.getElementById('offline-employees').textContent = offline;
    document.getElementById('employee-count').textContent = `Сотрудников онлайн: ${active}`;
}

function selectEmployee(username) {
    selectedEmployee = username;
    renderEmployeeList();
    loadEmployeeActivity();
    loadEmployeeStats();
    loadDailyReport();
}

async function loadEmployeeActivity() {
    if (!selectedEmployee) {
        document.getElementById('employee-activity').innerHTML = '<p>Выберите сотрудника из списка</p>';
        return;
    }
    
    const from = document.getElementById('date-from').value || new Date(Date.now() - 24*60*60*1000).toISOString();
    const to = document.getElementById('date-to').value || new Date().toISOString();
    
    try {
        const response = await fetch(`/api/activity/${encodeURIComponent(selectedEmployee)}?from=${from}&to=${to}`);
        const data = await response.json();
        
        const container = document.getElementById('employee-activity');
        
        if (!data || data.length === 0) {
            container.innerHTML = '<p>Нет данных за выбранный период</p>';
            return;
        }
        
        container.innerHTML = `
            <table>
                <thead>
                    <tr>
                        <th>Время</th>
                        <th>Окно</th>
                        <th>Процесс</th>
                        <th>Категория</th>
                        <th>Длительность</th>
                    </tr>
                </thead>
                <tbody>
                    ${data.map(item => `
                        <tr>
                            <td>${formatTime(item.timestamp)}</td>
                            <td>${item.window_title || 'Без названия'}</td>
                            <td>${item.process_name}</td>
                            <td>
                                <span class="category-badge" style="background: ${categoryColors[item.category] || categoryColors.neutral}">
                                    ${categoryNames[item.category] || 'Нейтрально'}
                                </span>
                            </td>
                            <td>${formatDuration(item.duration)}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    } catch (error) {
        console.error('Error loading employee activity:', error);
        document.getElementById('employee-activity').innerHTML = '<p>Ошибка загрузки данных</p>';
    }
}

async function loadEmployeeStats() {
    if (!selectedEmployee) {
        document.getElementById('app-stats').innerHTML = '<p>Выберите сотрудника из списка</p>';
        return;
    }
    
    const from = document.getElementById('date-from').value || new Date(Date.now() - 24*60*60*1000).toISOString();
    const to = document.getElementById('date-to').value || new Date().toISOString();
    
    try {
        const response = await fetch(`/api/activity/applications/${encodeURIComponent(selectedEmployee)}?start_time=${from}&end_time=${to}`);
        const data = await response.json();
        
        const container = document.getElementById('app-stats');
        
        if (!data || data.length === 0) {
            container.innerHTML = '<p>Нет статистики за выбранный период</p>';
            return;
        }
        
        const maxDuration = Math.max(...data.map(app => app.duration));
        
        const categoryTotals = {};
        data.forEach(app => {
            const cat = app.category || 'neutral';
            categoryTotals[cat] = (categoryTotals[cat] || 0) + app.duration;
        });
        
        const totalTime = Object.values(categoryTotals).reduce((a, b) => a + b, 0);
        
        let html = '<div class="category-summary">';
        html += '<h3>Распределение по категориям</h3>';
        html += '<div class="category-bars">';
        
        for (const [cat, duration] of Object.entries(categoryTotals).sort((a, b) => b[1] - a[1])) {
            const pct = totalTime > 0 ? (duration / totalTime * 100).toFixed(1) : 0;
            html += `
                <div class="category-bar-item">
                    <div class="category-bar-label">
                        <span class="category-badge" style="background: ${categoryColors[cat] || categoryColors.neutral}">
                            ${categoryNames[cat] || cat}
                        </span>
                        <span>${formatDuration(duration)} (${pct}%)</span>
                    </div>
                    <div class="category-bar-bg">
                        <div class="category-bar-fill" style="width: ${pct}%; background: ${categoryColors[cat] || categoryColors.neutral}"></div>
                    </div>
                </div>
            `;
        }
        html += '</div></div>';
        
        html += '<h3>Использование приложений</h3>';
        html += data.map(app => `
            <div class="chart-item">
                <div class="chart-label">
                    <span>
                        <span class="category-dot" style="background: ${categoryColors[app.category] || categoryColors.neutral}"></span>
                        ${app.process_name}
                    </span>
                    <span>${formatDuration(app.duration)} (${app.percentage ? app.percentage.toFixed(1) : 0}%)</span>
                </div>
                <div class="chart-bar" style="width: ${(app.duration / maxDuration) * 100}%; background: ${categoryColors[app.category] || categoryColors.neutral}"></div>
            </div>
        `).join('');
        
        container.innerHTML = html;
    } catch (error) {
        console.error('Error loading employee stats:', error);
        document.getElementById('app-stats').innerHTML = '<p>Ошибка загрузки статистики</p>';
    }
}

async function loadDailyReport() {
    if (!selectedEmployee) {
        const container = document.getElementById('daily-report');
        if (container) {
            container.innerHTML = '<p>Выберите сотрудника из списка</p>';
        }
        return;
    }
    
    const container = document.getElementById('daily-report');
    if (!container) return;
    
    const dateInput = document.getElementById('report-date');
    const date = dateInput ? dateInput.value : new Date().toISOString().split('T')[0];
    
    try {
        const response = await fetch(`/api/reports/daily/${encodeURIComponent(selectedEmployee)}?date=${date}`);
        const report = await response.json();
        
        if (!report || report.error) {
            container.innerHTML = '<p>Нет данных за выбранный день</p>';
            return;
        }
        
        let html = '<div class="daily-report-content">';
        
        html += `
            <div class="report-summary">
                <h3>Сводка за ${report.date}</h3>
                <div class="summary-stats">
                    <div class="summary-item">
                        <span class="summary-label">Всего активности:</span>
                        <span class="summary-value">${report.activity_events ? report.activity_events.length : 0} событий</span>
                    </div>
                    <div class="summary-item">
                        <span class="summary-label">Скриншотов:</span>
                        <span class="summary-value">${report.screenshots ? report.screenshots.length : 0}</span>
                    </div>
                    <div class="summary-item">
                        <span class="summary-label">USB событий:</span>
                        <span class="summary-value">${report.usb_events ? report.usb_events.length : 0}</span>
                    </div>
                    <div class="summary-item">
                        <span class="summary-label">Файловых операций:</span>
                        <span class="summary-value">${report.file_events ? report.file_events.length : 0}</span>
                    </div>
                </div>
            </div>
        `;
        
        if (report.activity_periods && report.activity_periods.length > 0) {
            html += '<div class="report-section"><h4>Хронология дня</h4><div class="timeline">';
            report.activity_periods.slice(0, 50).forEach(period => {
                const startTime = formatTimeShort(period.start);
                const endTime = formatTimeShort(period.end);
                const bgColor = period.state === 'idle' ? '#f5f5f5' : (categoryColors[period.category] || categoryColors.neutral);
                const textColor = period.state === 'idle' ? '#666' : '#fff';
                
                html += `
                    <div class="timeline-item" style="border-left-color: ${bgColor}">
                        <div class="timeline-time">${startTime} - ${endTime}</div>
                        <div class="timeline-content">
                            <span class="timeline-app">${period.friendly_name || period.process_name || period.state}</span>
                            <span class="timeline-duration">${formatDuration(period.duration_sec)}</span>
                            ${period.category ? `<span class="category-badge small" style="background: ${categoryColors[period.category]}">${categoryNames[period.category]}</span>` : ''}
                        </div>
                    </div>
                `;
            });
            html += '</div></div>';
        }
        
        if (report.applications && report.applications.length > 0) {
            html += '<div class="report-section"><h4>Топ приложений</h4><div class="apps-list">';
            const maxDur = Math.max(...report.applications.map(a => a.duration));
            report.applications.slice(0, 10).forEach(app => {
                html += `
                    <div class="app-item">
                        <div class="app-info">
                            <span class="category-dot" style="background: ${categoryColors[app.category] || categoryColors.neutral}"></span>
                            <span class="app-name">${app.process_name}</span>
                            <span class="app-category">${categoryNames[app.category] || 'Нейтрально'}</span>
                        </div>
                        <div class="app-bar-container">
                            <div class="app-bar" style="width: ${(app.duration / maxDur) * 100}%; background: ${categoryColors[app.category] || categoryColors.neutral}"></div>
                            <span class="app-duration">${formatDuration(app.duration)}</span>
                        </div>
                    </div>
                `;
            });
            html += '</div></div>';
        }
        
        if (report.screenshots && report.screenshots.length > 0) {
            html += '<div class="report-section"><h4>Скриншоты</h4><div class="screenshots-grid">';
            report.screenshots.slice(0, 12).forEach(ss => {
                html += `
                    <div class="screenshot-thumb" onclick="viewScreenshot('${ss.screenshot_id}')">
                        <div class="screenshot-time">${formatTimeShort(ss.timestamp)}</div>
                        <div class="screenshot-app">${ss.process_name || 'Unknown'}</div>
                    </div>
                `;
            });
            html += '</div></div>';
        }
        
        if (report.dlp_alerts && report.dlp_alerts.length > 0) {
            html += '<div class="report-section alerts-section"><h4>Предупреждения безопасности</h4><div class="alerts-list">';
            report.dlp_alerts.forEach(alert => {
                const severityClass = alert.severity === 'high' ? 'alert-high' : (alert.severity === 'medium' ? 'alert-medium' : 'alert-low');
                html += `
                    <div class="alert-item ${severityClass}">
                        <div class="alert-header">
                            <span class="alert-type">${alert.alert_type}</span>
                            <span class="alert-time">${formatTime(alert.timestamp)}</span>
                        </div>
                        <div class="alert-description">${alert.description}</div>
                    </div>
                `;
            });
            html += '</div></div>';
        }
        
        html += '</div>';
        container.innerHTML = html;
        
    } catch (error) {
        console.error('Error loading daily report:', error);
        container.innerHTML = '<p>Ошибка загрузки отчета</p>';
    }
}

function viewScreenshot(id) {
    window.open(`/api/screenshots/file/${id}`, '_blank');
}

function updateCurrentTime() {
    document.getElementById('current-time').textContent = new Date().toLocaleString('ru-RU');
}

document.addEventListener('DOMContentLoaded', () => {
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24*60*60*1000);
    
    document.getElementById('date-from').value = yesterday.toISOString().slice(0, 16);
    document.getElementById('date-to').value = now.toISOString().slice(0, 16);
    
    const reportDateInput = document.getElementById('report-date');
    if (reportDateInput) {
        reportDateInput.value = now.toISOString().split('T')[0];
    }
    
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tabName = btn.dataset.tab;
            
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            btn.classList.add('active');
            document.getElementById(tabName).classList.add('active');
            
            if (tabName === 'report' && selectedEmployee) {
                loadDailyReport();
            }
        });
    });
    
    document.getElementById('search-employee').addEventListener('input', renderEmployeeList);
    document.getElementById('status-filter').addEventListener('change', renderEmployeeList);
    
    fetchEmployees();
    fetchRecentActivity();
    updateCurrentTime();
    
    setInterval(() => {
        fetchEmployees();
        fetchRecentActivity();
        updateCurrentTime();
    }, 5000);
});
