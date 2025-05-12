document.addEventListener('DOMContentLoaded', function() { 
    loadDownloadHistory(); 

    async function loadDownloadHistory() { 
        try { 
            const response = await fetch('/history'); 
            if (!response.ok) {
                // 输出详细错误信息
                const errorText = await response.text();
                throw new Error(`获取历史记录失败: ${errorText}`);
            }
    
            const data = await response.json(); 
            if (data.code === 200) { 
                renderHistory(data.data); 
            } else { 
                showAlert(data.message, 'error'); 
            } 
        } catch (error) { 
            console.error(error); // 输出错误到控制台，方便调试
            showAlert(error.message, 'error'); 
        } 
    } 

    function renderHistory(history) { 
        const container = document.getElementById('historyList'); 
        container.innerHTML = history.map(item => ` 
            <div class="history-item"> 
                <div style="margin-bottom: 10px;"> 
                    <span class="status-badge status-${item.status}"> 
                        ${getStatusText(item.status)} 
                    </span> 
                    <span style="margin-left: 15px; font-weight: bold;"> 
                        ${formatTime(item.start_time)} 
                    </span> 
                </div> 
                <div style="margin-bottom: 8px;"> 
                    <strong>URL:</strong> 
                    <a href="${item.url}" target="_blank" style="color: #3498db;"> 
                        ${shortenURL(item.url, 50)} 
                    </a> 
                </div> 
                <div style="display: flex; justify-content: space-between;"> 
                    <div> 
                        <strong>文件类型:</strong> 
                        ${item.file_types.join(', ')} 
                    </div> 
                    <div class="time-info"> 
                        耗时: ${calcDuration(item.start_time, item.end_time)} 
                    </div> 
                </div> 
                <div class="progress-info" style="margin-top: 10px;"> 
                    完成 ${item.completed}/${item.total} 项（失败 ${item.failed}） 
                </div> 
            </div> 
        `).join(''); 
    } 

    // 公共函数 
    function shortenURL(url, maxLength) { 
        return url.length > maxLength ? url.substring(0, maxLength-3) + '...' : url; 
    } 

    function formatTime(timestamp) {
        // 尝试将时间戳转换为 Date 对象
        const date = new Date(timestamp);
        if (isNaN(date.getTime())) {
            return '无效时间';
        }
        return date.toLocaleString(); 
    } 

    function calcDuration(start, end) { 
        const startDate = new Date(start);
        if (isNaN(startDate.getTime())) {
            return '无效开始时间';
        }
        if (!end) return '进行中'; 
        const endDate = new Date(end);
        if (isNaN(endDate.getTime())) {
            return '无效结束时间';
        }
        const diff = endDate - startDate; 
        const mins = Math.floor(diff / 60000); 
        const secs = ((diff % 60000) / 1000).toFixed(0); 
        return `${mins}分${secs}秒`; 
    } 

    function getStatusText(status) { 
        return { 
            running: '进行中', 
            completed: '已完成', 
            cancelled: '已取消' 
        }[status] || '未知状态'; 
    } 

    function showAlert(message, type = 'info') { 
        const alert = document.createElement('div'); 
        alert.className = `alert-${type}`; 
        alert.textContent = message; 
        document.body.appendChild(alert); 
        setTimeout(() => alert.remove(), 3000); 
    } 
});