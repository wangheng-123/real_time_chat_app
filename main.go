package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"net/http"
)

type ClientManager struct {
	clients    map[*Client]bool //所有连接的客户端
	broadcast  chan []byte      //与所有连接的客户端广播的消息
	register   chan *Client     //尝试注册的客户端
	unregister chan *Client     //已销毁并等待删除的客户端
}

type Client struct {
	id     string          //唯一的 ID
	socket *websocket.Conn //一个套接字连接
	send   chan []byte     //一条等待发送的消息
}

type Message struct {
	Sender    string `json:"sender,omitempty"`    //消息发送者
	Recipient string `json:"recipient,omitempty"` //接收消息的人员
	Content   string `json:"content,omitempty"`   //消息的实际内容的信息
}

//启动一个全局ClientManager
var manager = ClientManager{
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

//manager.send:为了保存重复的代码，创建了一个方法来遍历每个客户端
func (manager *ClientManager) send(message []byte, ignore *Client) {
	for conn := range manager.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}

//服务器将使用三个 goroutine，一个用于管理客户端，一个用于读取 websocket 数据，一个用于写入 websocket 数据

//管理客户端
//manager.register:每次通道有数据时，客户端都会被添加到由客户端管理器管理的可用客户端映射中。添加客户端后，
//JSON 消息将发送到所有其他客户端，不包括刚刚连接的客户端。
//
//manager.unregister:如果客户端因任何原因断开连接，通道将具有数据。断开连接的客户端中的通道数据将被关闭，
//客户端将从客户端管理器中删除。宣布套接字消失的消息将发送到所有剩余的连接。
//
//manager.broadcast:如果通道有数据，则意味着我们正在尝试发送和接收消息。我们希望遍历每个托管客户端，
//将消息发送给每个客户端。如果由于某种原因通道堵塞或无法发送消息，我们假设客户端已断开连接，我们将删除它们。
func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			manager.send(jsonMessage, conn)
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.send(jsonMessage, conn)
			}
		case message := <-manager.broadcast:
			for conn := range manager.clients {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(manager.clients, conn)
				}
			}
		}
	}
}

//现在我们可以探索用于读取从客户端发送的 websocket 数据的 goroutine。
//此 goroutine的重点是读取套接字数据并将其添加到 manager.broadcast 中以进行进一步的编排。
//如果读取 websocket 数据时出错，则可能意味着客户端已断开连接。如果是这种情况，我们需要从服务器中注销客户端。
func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()

	for {
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
		manager.broadcast <- jsonMessage
	}
}

//c.send:如果通道有数据，我们会尝试发送消息。如果由于某种原因通道不正常，我们将向客户端发送断开连接消息。
func (c *Client) write() {
	defer func() {
		c.socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

//HTTP 请求使用 websocket 库升级到 websocket 请求。通过添加 我们可以接受来自外部域的请求，
//从而消除跨源资源共享 （CORS） 错误。CheckOrigin
//建立连接时，将创建客户端并生成唯一 ID。如前所述，此客户端已注册到服务器。
//客户端注册后，将触发读取和写入 goroutine。
func wsPage(res http.ResponseWriter, req *http.Request) {
	conn, error := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if error != nil {
		http.NotFound(res, req)
		return
	}
	client := &Client{id: uuid.NewV4().String(), socket: conn, send: make(chan []byte)}

	manager.register <- client

	go client.read()
	go client.write()
}

//那么我们如何开始这些goroutine中的每一个呢？服务器 goroutine 将在我们启动服务器时启动，
//其他每个 goroutines 将在有人连接时启动。
//我们在端口 12345 上启动服务器，它有一个只能通过 websocket 连接访问的端点。
func main() {
	fmt.Println("Starting application...")
	go manager.start()
	http.HandleFunc("/ws", wsPage)
	http.ListenAndServe(":12345", nil)
}
