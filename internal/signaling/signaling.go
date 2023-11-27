package signaling

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

type WSType string

const (
	CONNECTED WSType = "Connected"
	SDP       WSType = "SDP"
	ICE       WSType = "ICE"
	ERROR     WSType = "Error"
)

type WsMsg struct {
	Sender bool
	WSType WSType
	SDP    string
	Answer string
	Data   string
}

func NewWsMsg() *WsMsg {
	return &WsMsg{
		Sender: true,
		WSType: CONNECTED,
		SDP:    "",
		Answer: "",
		Data:   "",
	}
}

type Signaling struct {
	TIMESWAIT    int
	TIMESWAITMAX int
	wsConn       *websocket.Conn
}

func (sig *Signaling) Init(urlStr string) error {
	c, _, err := websocket.DefaultDialer.Dial(urlStr, nil)
	if err != nil {
		return err
	}
	sig.wsConn = c
	return nil
}

func (sig *Signaling) SendMsg(data *WsMsg) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return sig.wsConn.WriteMessage(websocket.TextMessage, jsonData)
}

func (sig *Signaling) ReadMsg() (*WsMsg, error) {
	_, message, err := sig.wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}
	nmsg := NewWsMsg()
	if err := json.Unmarshal(message, nmsg); err != nil {
		return nil, err
	}
	return nmsg, nil
}

func (sig *Signaling) Close() error {
	return sig.wsConn.Close()
}
