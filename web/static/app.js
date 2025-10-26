let selectedEmployee = null;
let employees = [];
let activities = [];

function formatTime(date) {
    return new Date(date).toLocaleString('ru-RU');
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
            <div class="activity-process">Процесс: ${activity.process_name} (${formatDuration(activity.duration)})</div>
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
}

async function loadEmployeeActivity() {
    if (!selectedEmployee) {
        document.getElementById('employee-activity').innerHTML = '<p>Выберите сотрудника из списка</p>';
        return;
    }
    
    const from = document.getElementById('date-from').value || new Date(Date.now() - 24*60*60*1000).toISOString();
    const to = document.getElementById('date-to').value || new Date().toISOString();
    
    try {
        const response = await fetch(`/api/activity/${selectedEmployee}?from=${from}&to=${to}`);
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
                        <th>Длительность</th>
                    </tr>
                </thead>
                <tbody>
                    ${data.map(item => `
                        <tr>
                            <td>${formatTime(item.timestamp)}</td>
                            <td>${item.window_title || 'Без названия'}</td>
                            <td>${item.process_name}</td>
                            <td>${formatDuration(item.duration)}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    } catch (error) {
        console.error('Error loading employee activity:', error);
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
        const response = await fetch(`/api/stats/${selectedEmployee}?from=${from}&to=${to}`);
        const data = await response.json();
        
        const container = document.getElementById('app-stats');
        
        if (!data || Object.keys(data).length === 0) {
            container.innerHTML = '<p>Нет статистики за выбранный период</p>';
            return;
        }
        
        const maxDuration = Math.max(...Object.values(data));
        
        container.innerHTML = Object.entries(data)
            .sort((a, b) => b[1] - a[1])
            .map(([app, duration]) => `
                <div class="chart-item">
                    <div class="chart-label">
                        <span>${app}</span>
                        <span>${formatDuration(duration)}</span>
                    </div>
                    <div class="chart-bar" style="width: ${(duration / maxDuration) * 100}%"></div>
                </div>
            `).join('');
    } catch (error) {
        console.error('Error loading employee stats:', error);
    }
}

function updateCurrentTime() {
    document.getElementById('current-time').textContent = new Date().toLocaleString('ru-RU');
}

document.addEventListener('DOMContentLoaded', () => {
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24*60*60*1000);
    
    document.getElementById('date-from').value = yesterday.toISOString().slice(0, 16);
    document.getElementById('date-to').value = now.toISOString().slice(0, 16);
    
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tabName = btn.dataset.tab;
            
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            btn.classList.add('active');
            document.getElementById(tabName).classList.add('active');
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
