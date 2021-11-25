package websocket

import (
	"encoding/json"
	"github.com/eric2788/biligo-live-ws/services/blive"
	"github.com/eric2788/biligo-live-ws/services/subscriber"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	live "github.com/iyear/biligo-live"
	"log"
)

var websocketTable map[string]*websocket.Conn

func Register(gp *gin.RouterGroup) {
	gp.GET("", OpenWebSocket)
	go blive.SubscribedRoomTracker(handleBLiveMessage)
}

func OpenWebSocket(c *gin.Context) {
	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		_ = c.Error(err)
		return
	}
	websocketTable[c.ClientIP()] = ws
}

func handleBLiveMessage(room int64, info blive.LiveInfo, msg live.Msg) {
	if _, ok := msg.(*live.MsgHeartbeatReply); ok { // heartbeat 訊息
		return
	}
	bLiveData := BLiveData{
		Command:  msg.Cmd(),
		LiveInfo: info,
		Content:  msg.Raw(),
	}
	body := string(msg.Raw())
	log.Printf("從 %v 收到訊息: %v\n", room, body)

	for _, ip := range subscriber.GetAllSubscribers(room) {
		if err := writeMessage(ip, bLiveData); err != nil {
			log.Printf("向 用戶 %v 發送直播數據時出現錯誤: %v\n", ip, err)
		}
	}
}

func writeMessage(ip string, data BLiveData) error {
	con, ok := websocketTable[ip]

	if !ok {
		log.Println("Trying to send websocket with unknown ip: ", ip)
		subscriber.Delete(ip)
		return nil
	}
	byteData, err := json.Marshal(data)

	if err != nil {
		return err
	}

	if err = con.WriteMessage(websocket.TextMessage, byteData); err != nil {
		switch err.(type) {
		case *websocket.CloseError: // if socket closed
			delete(websocketTable, ip)
			subscriber.Delete(ip)
		default:
			return err
		}
	}

	return nil
}

type BLiveData struct {
	Command  string         `json:"command"`
	LiveInfo blive.LiveInfo `json:"live_info"`
	Content  []byte         `json:"content"`
}
