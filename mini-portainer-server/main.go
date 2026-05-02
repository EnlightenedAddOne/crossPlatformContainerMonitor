package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// --- 数据结构 ---
type ContainerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	State  string `json:"state"`
}

type SystemStatus struct {
	CPUTemp    float64         `json:"cpu_temp"`
	MemTotalMB int             `json:"mem_total_mb"`
	MemUsedMB  int             `json:"mem_used_mb"`
	Containers []ContainerInfo `json:"containers"`
	LastUpdate time.Time       `json:"last_update"`
}

type Command struct {
	Action      string `json:"action"`
	ContainerID string `json:"container_id"`
}

var (
	nodeData = make(map[string]SystemStatus)
	cmdQueue = make(map[string]*Command)

	// 🗄️ 新增：日志暂存区。Key 是 "节点名_容器ID"，Value 是具体的日志文本
	logCache = make(map[string]string)

	mu sync.RWMutex
)

func main() {
	// 接口 1：接收状态报告与下发信件 (保持不变)
	http.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		nodeName := r.URL.Query().Get("node")
		if nodeName == "" {
			nodeName = "Unknown"
		}

		var status SystemStatus
		json.NewDecoder(r.Body).Decode(&status)
		status.LastUpdate = time.Now()

		mu.Lock()
		nodeData[nodeName] = status
		cmd := cmdQueue[nodeName]
		if cmd != nil {
			delete(cmdQueue, nodeName)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if cmd != nil {
			json.NewEncoder(w).Encode(cmd)
		} else {
			json.NewEncoder(w).Encode(Command{Action: "none"})
		}
	})

	// 接口 2：前端获取大盘数据 (保持不变)
	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodeData)
	})

	// 接口 3：前端下发控制指令 (保持不变)
	http.HandleFunc("/api/command", func(w http.ResponseWriter, r *http.Request) {
		node := r.URL.Query().Get("node")
		action := r.URL.Query().Get("action")
		cid := r.URL.Query().Get("id")

		if node != "" && action != "" && cid != "" {
			mu.Lock()
			cmdQueue[node] = &Command{Action: action, ContainerID: cid}
			mu.Unlock()
			fmt.Printf("🎯 指令入匣: 节点 [%s] 执行 [%s] 容器 [%s]\n", node, action, cid)
		}
		w.WriteHeader(http.StatusOK)
	})

	// 🚀 接口 4 (新增)：Agent 上传提取好的日志
	http.HandleFunc("/api/logs/submit", func(w http.ResponseWriter, r *http.Request) {
		node := r.URL.Query().Get("node")
		cid := r.URL.Query().Get("id")

		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		logCache[node+"_"+cid] = string(body)
		mu.Unlock()

		fmt.Printf("📦 收到节点 [%s] 容器 [%s] 的日志回传！\n", node, cid)
		w.WriteHeader(http.StatusOK)
	})

	// 🚀 接口 5 (新增)：前端轮询获取日志
	http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		node := r.URL.Query().Get("node")
		cid := r.URL.Query().Get("id")
		cacheKey := node + "_" + cid

		mu.RLock()
		logs, exists := logCache[cacheKey]
		mu.RUnlock()

		if exists {
			w.Write([]byte(logs))
			// 阅后即焚，释放内存
			mu.Lock()
			delete(logCache, cacheKey)
			mu.Unlock()
		} else {
			// 如果还没准备好，返回 404，前端会继续轮询等待
			http.Error(w, "Logs not ready", http.StatusNotFound)
		}
	})

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	fmt.Println("☁️ 云端 API (新增快照日志功能) 已启动！监听: 9000")
	http.ListenAndServe(":9000", nil)
}
