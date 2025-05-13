document.addEventListener('DOMContentLoaded', function() {
    const urlInput = document.getElementById('urlInput');
    const downloadBtn = document.getElementById('downloadBtn');
    const previewBtn = document.getElementById('previewBtn');
    const cancelBtn = document.getElementById('cancelBtn');
    const progressContainer = document.getElementById('progressContainer');
    const progressBar = document.getElementById('progressBar');
    const completedTasksElement = document.getElementById('completedTasks');
    const totalTasksElement = document.getElementById('totalTasks');
    const failedTasksElement = document.getElementById('failedTasks');
    const downloadRateElement = document.getElementById('downloadRate');
    const taskListElement = document.getElementById('taskList');
    const fileListContainer = document.getElementById('fileListContainer');
    const fileListElement = document.getElementById('fileList');
    const loading = document.createElement('div');
    loading.className = 'loading';
    loading.innerHTML = '<div class="loading-spinner"></div>';

    let eventSource = null;
    let downloadInProgress = false;
    let previewedFiles = [];

    // 显示加载动画
    function showLoading() {
        document.body.appendChild(loading);
    }

    // 隐藏加载动画
    function hideLoading() {
        if (loading.parentNode) {
            loading.parentNode.removeChild(loading);
        }
    }

    // 文件类型选择
    function getSelectedFileTypes() {
        const checkboxes = document.querySelectorAll('.file-types input[type="checkbox"]:checked');
        return Array.from(checkboxes).map(checkbox => checkbox.value);
    }

    // 查看资源
    previewBtn.addEventListener('click', function() {
        const url = urlInput.value.trim();
        if (!url) {
            showAlert('请输入有效的网址', 'warning');
            return;
        }
    
        const fileTypes = getSelectedFileTypes();
    
        fetch('/preview', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, file_types: fileTypes })
        })
            .then(handleResponse)
            .then(data => {
                if (data.data?.tasks?.length > 0) {
                    previewedFiles = data.data.tasks;
                    renderFileList(previewedFiles);
                    fileListContainer.style.display = 'block';
                    // 统计资源分类数量
                const countMap = countResourceTypes(previewedFiles);
                renderResourceCount(countMap);
            } else {
                showAlert('未找到任何资源', 'info');
            }
        })
        .catch(handleError);
    });

// 统计资源分类数量
function countResourceTypes(tasks) {
    const countMap = {};
    tasks.forEach(task => {
        if (task.type) {
            if (countMap[task.type]) {
                countMap[task.type]++;
            } else {
                countMap[task.type] = 1;
            }
        }
    });
    return countMap;
}

// 渲染资源分类统计信息
function renderResourceCount(countMap) {
    const resourceCountList = document.getElementById('resourceCountList');
    if (Object.keys(countMap).length === 0) {
        resourceCountList.innerHTML = '<li>暂无分类统计信息</li>';
        return;
    }
    resourceCountList.innerHTML = Object.entries(countMap).map(([type, count]) => `
        <li>
            <span>${type}:</span>
            <span>${count} 个</span>
        </li>
    `).join('');
}
    
    // 开始下载
    downloadBtn.addEventListener('click', function() {
        const url = urlInput.value.trim();
        if (!url) {
            showAlert('请输入有效的网址', 'warning');
            return;
        }
        showLoading();
        initDownloadUI();
        startDownload(url);
    });

    // 取消下载
    cancelBtn.addEventListener('click', function() {
        showLoading();
        fetch('/cancel')
            .then(handleResponse)
            .then(() => {
                hideLoading();
                resetDownloadUI();
                showAlert('下载已取消', 'success');
            })
            .catch(error => {
                hideLoading();
                handleError(error);
            });
    });

    // 初始化下载UI状态
    function initDownloadUI() {
        progressContainer.style.display = 'block';
        cancelBtn.style.display = 'inline-block';
        downloadBtn.style.display = 'none';
        previewBtn.style.display = 'none';

        progressBar.style.width = '0%';
        completedTasksElement.textContent = '0';
        totalTasksElement.textContent = '0';
        failedTasksElement.textContent = '0';
        downloadRateElement.textContent = '0';
        taskListElement.innerHTML = '';
    }

    // 启动下载流程
    function startDownload(url) {
        const fileTypes = getSelectedFileTypes();

        // 先关闭旧的SSE连接
        if (eventSource) {
            eventSource.close();
        }

        // 建立新的SSE连接
        eventSource = new EventSource(`/progress-sse?timestamp=${Date.now()}`);

        eventSource.onmessage = (event) => {
            const data = JSON.parse(event.data);
            updateProgressUI(data);
        };

        eventSource.onerror = (error) => {
            console.error('SSE Error:', error);
            eventSource.close();
            if (downloadInProgress) {
                showAlert('连接中断，正在尝试重连...', 'error');
                setTimeout(() => startDownload(url), 3000);
            }
        };

        // 发送下载请求
        fetch('/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, file_types: fileTypes })
        })
            .then(handleResponse)
            .then(() => {
                hideLoading();
            })
            .catch(error => {
                hideLoading();
                eventSource.close();
                resetDownloadUI();
                handleError(error);
            });

        downloadInProgress = true;
    }

    // 更新进度UI
    function updateProgressUI(progress) {
        // 更新总任务数（只在首次接收时更新）
        if (parseInt(totalTasksElement.textContent) === 0) {
            totalTasksElement.textContent = progress.total;
        }

        // 更新进度条
        const percentage = (progress.completed / progress.total) * 100;
        progressBar.style.width = `${percentage}%`;

        // 更新统计数字
        completedTasksElement.textContent = progress.completed;
        failedTasksElement.textContent = progress.failed;
        downloadRateElement.textContent = progress.rate.toFixed(2);

        // 更新任务列表
        taskListElement.innerHTML = '';
        progress.tasks.forEach(task => {
            const taskElement = document.createElement('li');
            taskElement.className = 'task-item';
            taskElement.innerHTML = `
                <span>${shortenURL(task.url, 50)}</span>
                <span class="task-status status-${task.status}">
                    ${getStatusText(task.status)}
                </span>
            `;
            taskListElement.appendChild(taskElement);
        });

        // 检查完成状态
        if (progress.completed + progress.failed >= progress.total) {
            downloadInProgress = false;
            eventSource.close();
            resetDownloadUI();
            showAlert('下载任务完成!', 'success');

        // 下载完成后统计并渲染资源分类
        const countMap = countResourceTypes(progress.tasks);
        renderResourceCount(countMap);
        }
    }

    // 重置下载UI
    function resetDownloadUI() {
        downloadInProgress = false;
        progressContainer.style.display = 'none';
        cancelBtn.style.display = 'none';
        downloadBtn.style.display = 'inline-block';
        previewBtn.style.display = 'inline-block';
        if (eventSource) eventSource.close();
    }

    // 辅助函数
    function handleResponse(response) {
        if (!response.ok) {
            return response.json().then(err => Promise.reject(err));
        }
        return response.json();
    }

    function handleError(error) {
        console.error('Error:', error);
        const message = error.message || error.msg || '请求失败';
        showAlert(message, 'error');
    }

    function showAlert(message, type = 'info') {
        const alertBox = document.createElement('div');
        alertBox.className = `alert-${type}`;
        alertBox.textContent = message;
        document.body.appendChild(alertBox);
        setTimeout(() => {
            if (alertBox.parentNode) {
                alertBox.parentNode.removeChild(alertBox);
            }
        }, 3000);
    }

    function renderFileList(files) {
        fileListElement.innerHTML = files.length ?
            files.map(file => `
                <li>
                    <strong>${file.filename}</strong>
                    <span class="file-type-badge">${file.type}</span>
                </li>
            `).join('') :
            '<li>未找到任何资源</li>';
    }

    function shortenURL(url, maxLen) {
        if (url.length <= maxLen) {
            return url;
        }
        return url.substring(0, maxLen - 3) + "...";
    }

    function getStatusText(status) {
        const statusMap = {
            pending: '等待中',
            downloading: '下载中',
            completed: '已完成',
            failed: '失败',
            skipped: '已跳过'
        };
        return statusMap[status] || status;
    }
});
