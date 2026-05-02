const { createApp, ref, onMounted, onUnmounted, computed } = Vue;

createApp({
    setup() {
        const nodes = ref({});
        const isConnected = ref(true);
        const countdown = ref(3);
        const currentLang = ref(localStorage.getItem('lang') || 'zh');

        // 扩充了操作按钮的翻译
        const i18n = {
            zh: {
                title: "MINI PORTAINER 全局控制台",
                apiOnline: "API 在线",
                apiOffline: "API 离线",
                nodes: "监控节点",
                refresh: "自动刷新",
                sync: "同步于",
                cpuTemp: "CPU 温度",
                ramUsage: "内存占用",
                containers: "容器列表",
                colName: "名称",
                colId: "ID",
                colStatus: "状态",
                colAction: "操作",
                btnStart: "启动容器",
                btnStop: "停止容器",
                btnRestart: "重启容器",
                waiting: "正在等待 Agent 节点上报数据...",
                switchLang: "English"
            },
            en: {
                title: "MINI PORTAINER GLOBAL",
                apiOnline: "API Online",
                apiOffline: "API Offline",
                nodes: "Nodes",
                refresh: "Auto Refresh",
                sync: "Sync",
                cpuTemp: "CPU TEMP",
                ramUsage: "RAM USAGE",
                containers: "Containers",
                colName: "Name",
                colId: "ID",
                colStatus: "Status",
                colAction: "Actions",
                btnStart: "Start",
                btnStop: "Stop",
                btnRestart: "Restart",
                waiting: "Waiting for Edge Agents to report...",
                switchLang: "中文"
            }
        };

        const t = computed(() => i18n[currentLang.value]);

        const toggleLang = () => {
            currentLang.value = currentLang.value === 'zh' ? 'en' : 'zh';
            localStorage.setItem('lang', currentLang.value);
        };

        const fetchData = async () => {
            try {
                const response = await fetch('/api/data');
                if (!response.ok) throw new Error();
                const data = await response.json();
                nodes.value = data || {};
                isConnected.value = true;
                countdown.value = 3;
            } catch (error) {
                isConnected.value = false;
            }
        };

        // 新增：发送指令给后端的信箱
        const sendCommand = async (node, action, id) => {
            // 简单加个防误触确认
            const actionText = currentLang.value === 'zh' ? 
                (action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启') : action;
            
            if (!confirm(`确定要 ${actionText} 该容器吗？`)) return;

            try {
                // 向云端发指令，云端会把它存在内存信箱里
                const res = await fetch(`/api/command?node=${node}&action=${action}&id=${id}`, { method: 'POST' });
                if (res.ok) {
                    // 可以加个轻提示，为了简单我们直接打印在控制台
                    console.log(`✅ 已将 ${action} 指令放入 ${node} 的信箱，等待节点取件...`);
                }
            } catch (e) {
                alert("指令发送失败，请检查网络！");
            }
        };

        const formatTime = (timeStr) => {
            if (!timeStr) return '--:--:--';
            const d = new Date(timeStr);
            return d.toLocaleTimeString(currentLang.value === 'zh' ? 'zh-CN' : 'en-US', { hour12: false });
        };

        const getCtrIcon = (name) => {
            name = name.toLowerCase();
            if (name.includes('adguard')) return 'shield-half-outline';
            if (name.includes('jiuguan')) return 'chatbubbles-outline';
            if (name.includes('nginx')) return 'globe-outline';
            if (name.includes('db') || name.includes('mysql')) return 'server-outline';
            return 'cube-outline';
        };

        // 🌟 日志弹窗相关
        const logModal = ref({ show: false, node: '', containerId: '', containerName: '', content: '' });
        let logInterval = null;

        // 查看日志的核心逻辑
        const viewLogs = async (node, ctr) => {
            // 1. 初始化弹窗
            logModal.value = { show: true, node: node, containerId: ctr.id, containerName: ctr.name, content: '' };
            
            // 2. 往云端信箱里塞入获取日志的指令
            await fetch(`/api/command?node=${node}&action=logs&id=${ctr.id}`, { method: 'POST' });

            // 3. 开始每隔 1 秒去前台问大哥："日志送回来了没？"
            let attempts = 0;
            logInterval = setInterval(async () => {
                attempts++;
                if (attempts > 12) { // 12秒超时（Agent 如果挂了就结束）
                    clearInterval(logInterval);
                    logModal.value.content = "⚠️ 获取日志超时，节点 Agent 可能已离线或网络拥堵。";
                    return;
                }
                try {
                    const res = await fetch(`/api/logs?node=${node}&id=${ctr.id}`);
                    if (res.ok) {
                        // 如果拿到了，填入内容并停止轮询
                        logModal.value.content = await res.text();
                        clearInterval(logInterval);
                        
                        // 自动滚动到底部
                        setTimeout(() => {
                            const box = document.getElementById('term-body');
                            if(box) box.scrollTop = box.scrollHeight;
                        }, 100);
                    }
                } catch (e) {}
            }, 1000);
        };

        const closeLogs = () => {
            logModal.value.show = false;
            if (logInterval) clearInterval(logInterval);
        };

        onMounted(() => {
            fetchData();
            setInterval(fetchData, 3000);
            setInterval(() => { if (countdown.value > 0) countdown.value--; }, 1000);
        });

        return { nodes, isConnected, countdown, t, toggleLang, currentLang, formatTime, getCtrIcon, sendCommand, logModal, viewLogs, closeLogs };
    }
}).mount('#app');