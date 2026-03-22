// Fetch and render data
document.addEventListener('DOMContentLoaded', async () => {
    await loadSummary();
    await loadTrends();
    await loadRecentRequests();
    
    // Auto-refresh every 30 seconds
    setInterval(async () => {
        await loadSummary();
        await loadRecentRequests();
    }, 30000);
});

async function loadSummary() {
    try {
        const response = await fetch('/api/metrics/summary');
        const stats = await response.json();
        
        updateStat('total-requests', stats.total);
        updateStat('today-requests', stats.today_total || 0);
        updateStat('avg-latency', (stats.avg_latency_ms || 0).toFixed(0) + ' ms');
        updateStat('tokens-per-sec', (stats.tokens_per_second || 0).toFixed(1));
        updateStat('today-tokens-in', stats.today_tokens_in || 0);
        updateStat('today-tokens-out', stats.today_tokens_out || 0);
        updateStat('avg-tokens-in', (stats.avg_tokens_in || 0).toFixed(0));
        updateStat('avg-tokens-out', (stats.avg_tokens_out || 0).toFixed(0));
    } catch (error) {
        console.error('Failed to load summary:', error);
    }
}

async function loadTrends() {
    try {
        const response = await fetch('/api/metrics/trends?interval=hour');
        const data = await response.json();
        
        renderRequestsChart(data);
        renderLatencyChart(data);
        renderTokensChart(data);
    } catch (error) {
        console.error('Failed to load trends:', error);
    }
}

async function loadRecentRequests() {
    try {
        const response = await fetch('/api/requests?limit=20');
        const requests = await response.json();
        
        const tbody = document.getElementById('requestsTableBody');
        tbody.innerHTML = '';
        
        requests.forEach(req => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${formatTime(req.created_at)}</td>
                <td>${req.client_ip}</td>
                <td>${req.model || '-'}</td>
                <td>${req.tokens_input || 0}</td>
                <td>${req.tokens_output || 0}</td>
                <td>${req.duration_ms ? req.duration_ms + ' ms' : '-'}</td>
                <td class="${req.status_code >= 200 && req.status_code < 300 ? 'status-ok' : 'status-error'}">
                    ${req.status_code || '-'}
                </td>
            `;
            tbody.appendChild(row);
        });
    } catch (error) {
        console.error('Failed to load recent requests:', error);
    }
}

function updateStat(elementId, value) {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = value;
    }
}

function formatTime(timestamp) {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { 
        hour: '2-digit', 
        minute: '2-digit',
        second: '2-digit'
    });
}

function renderRequestsChart(data) {
    const ctx = document.getElementById('requestsChart');
    if (!ctx) return;
    
    if (window.requestsChartInstance) {
        window.requestsChartInstance.destroy();
    }
    
    const labels = data.labels || [];
    const counts = data.data?.map(d => d.count) || [];
    
    window.requestsChartInstance = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'Requests',
                data: counts,
                borderColor: '#667eea',
                backgroundColor: 'rgba(102, 126, 234, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    display: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1
                    }
                }
            }
        }
    });
}

function renderLatencyChart(data) {
    const ctx = document.getElementById('latencyChart');
    if (!ctx) return;
    
    if (window.latencyChartInstance) {
        window.latencyChartInstance.destroy();
    }
    
    const labels = data.labels || [];
    const latencies = data.data?.map(d => d.avg_latency) || [];
    
    window.latencyChartInstance = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'Avg Latency (ms)',
                data: latencies,
                borderColor: '#f59e0b',
                backgroundColor: 'rgba(245, 158, 11, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    display: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true
                }
            }
        }
    });
}

function renderTokensChart(data) {
    const ctx = document.getElementById('tokensChart');
    if (!ctx) return;
    
    if (window.tokensChartInstance) {
        window.tokensChartInstance.destroy();
    }
    
    const labels = data.labels || [];
    const tokensIn = data.data?.map(d => d.tokens_in) || [];
    const tokensOut = data.data?.map(d => d.tokens_out) || [];
    
    window.tokensChartInstance = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Input Tokens',
                    data: tokensIn,
                    backgroundColor: 'rgba(16, 185, 129, 0.7)',
                },
                {
                    label: 'Output Tokens',
                    data: tokensOut,
                    backgroundColor: 'rgba(59, 130, 246, 0.7)',
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            scales: {
                y: {
                    beginAtZero: true
                }
            }
        }
    });
}

let sortDirection = true;
function sortTable(columnIndex) {
    const table = document.getElementById('requestsTable');
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));
    
    sortDirection = !sortDirection;
    
    rows.sort((a, b) => {
        const aCell = a.cells[columnIndex].textContent.trim();
        const bCell = b.cells[columnIndex].textContent.trim();
        
        if (!aCell || !bCell) return 0;
        
        // Try numeric comparison
        const aNum = parseFloat(aCell);
        const bNum = parseFloat(bCell);
        
        if (!isNaN(aNum) && !isNaN(bNum)) {
            return sortDirection ? aNum - bNum : bNum - aNum;
        }
        
        // String comparison
        return sortDirection 
            ? aCell.localeCompare(bCell)
            : bCell.localeCompare(aCell);
    });
    
    rows.forEach(row => tbody.appendChild(row));
}
