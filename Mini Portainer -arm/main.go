package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
}

// 📦 新增：解析云端返回的指令字条
type CommandResponse struct {
	Action      string `json:"action"`
	ContainerID string `json:"container_id"`
}

// ==========================================
// ⚠️ 修改这里：填入你刚才部署 Server 的阿里云公网 IP
// ==========================================
const CloudServerURL = "http://阿里云公网IP:9000/api/report?node=RaspberryPi_3B"




// --- 硬件采集 ---
func getCPUTemp() float64 {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0.0
	}
	tempStr := strings.TrimSpace(string(data))
	tempInt, _ := strconv.Atoi(tempStr)
	return float64(tempInt) / 1000.0
}

func getMemoryInfo() (int, int) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(data), "\n")
	var memTotal, memAvailable int
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d kB", &memTotal)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d kB", &memAvailable)
		}
	}
	return memTotal / 1024, (memTotal - memAvailable) / 1024
}

func executeDockerCommand(cli *client.Client, action string, containerID string) {
	ctx := context.Background()

	// 处理获取日志的特殊指令
	if action == "logs" {
		fmt.Printf("📄 正在拉取容器 [%s] 的日志...\n", containerID)
		// 抓取最后 200 行日志
		out, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: "200"})
		if err != nil {
			fmt.Println("❌ 获取日志失败:", err)
			return
		}
		defer out.Close()

		rawData, _ := io.ReadAll(out)
		
		// 魔法过滤：Docker 在非 TTY 模式下每行会有 8 字节的二进制报头，会导致乱码。
		// 我们通过保留 ASCII 大于 31 的可见字符（包括中文字符）以及换行符，巧妙地把报头剔除干净。
		var cleanLogs strings.Builder
		for _, runeValue := range string(rawData) {
			if runeValue >= 32 || runeValue == '\n' || runeValue == '\r' || runeValue == '\t' {
				cleanLogs.WriteRune(runeValue)
			}
		}

		logStr := cleanLogs.String()
		if logStr == "" { logStr = "> 该容器暂无日志输出或日志被清空。" }

		// 把日志提交给云端的新接口
		submitURL := strings.Replace(CloudServerURL, "/api/report", "/api/logs/submit", 1) + "&id=" + containerID
		http.Post(submitURL, "text/plain", strings.NewReader(logStr))
		fmt.Println("✅ 日志已成功送达云端！")
		return
	}

	// 原有的启停逻辑
	fmt.Printf("⚙️  正在执行操作: [%s] 容器 [%s]...\n", action, containerID)
	var err error
	switch action {
	case "start": err = cli.ContainerStart(ctx, containerID, container.StartOptions{})
	case "stop": err = cli.ContainerStop(ctx, containerID, container.StopOptions{})
	case "restart": err = cli.ContainerRestart(ctx, containerID, container.StopOptions{})
	}
	if err != nil {
		fmt.Printf("❌ 执行失败: %v\n", err)
	} else {
		fmt.Printf("✅ 执行成功！\n")
	}
}

// --- 主程序 ---
func main() {
	fmt.Println("🚀 Mini Portainer Agent (全功能版) 正在启动...")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("无法连接到 Docker: %v", err)
	}
	defer cli.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("📡 开始向云端通信: %s\n", CloudServerURL)

	for range ticker.C {
		// 1. 采集状态
		cpuTemp := getCPUTemp()
		totalMem, usedMem := getMemoryInfo()

		containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			fmt.Printf("❌ 获取容器失败: %v\n", err)
			continue
		}

		var ctrList []ContainerInfo
		for _, ctr := range containers {
			name := ctr.Names[0][1:]
			ctrList = append(ctrList, ContainerInfo{
				ID:     ctr.ID[:10],
				Name:   name,
				Status: ctr.Status,
				State:  ctr.State,
			})
		}

		status := SystemStatus{
			CPUTemp:    cpuTemp,
			MemTotalMB: totalMem,
			MemUsedMB:  usedMem,
			Containers: ctrList,
		}

		jsonData, _ := json.Marshal(status)

		// 2. 发送状态给云端大哥
		resp, err := http.Post(CloudServerURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("⚠️ 无法连接云端: %v\n", err)
			continue
		}

		// 3. 拆开大哥塞回来的信封 (解析响应)
		var cmd CommandResponse
		if err := json.NewDecoder(resp.Body).Decode(&cmd); err == nil {
			// 如果信封里有实质性的指令，就立刻叫 Docker 去干活！
			if cmd.Action != "" && cmd.Action != "none" {
				fmt.Printf("\n📬 收到云端急件！\n")
				executeDockerCommand(cli, cmd.Action, cmd.ContainerID)
			}
		} else {
			fmt.Printf("⚠️ 解析云端响应失败: %v\n", err)
		}
		
		// 必须关掉响应体，防止内存泄漏
		resp.Body.Close()

		// 静默打印心跳，防止刷屏太乱
		fmt.Printf(".")
	}
}