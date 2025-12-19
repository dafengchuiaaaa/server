package hooks

import (
	"bytes"
	"log/slog"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// ConnectHook 用于连接时候的钩子
// 本钩子在连接建立时（OnConnect阶段）使用内联client发送主题消息
type ConnectHook struct {
	mqtt.HookBase
	connectTopic    string
	disConnectTopic string
	qos             byte
	server          *mqtt.Server
}

// NewConnectHook 创建一个新的连接钩子
func NewConnectHook(server *mqtt.Server) *ConnectHook {
	h := &ConnectHook{
		connectTopic:    "sys/connect",
		disConnectTopic: "sys/disconnect",
		qos:             1,
		server:          server,
	}
	return h
}

// ID 返回 Hook 的 ID
func (h *ConnectHook) ID() string {
	return "client-connect"
}

// Provides 返回 Hook 提供的功能
func (h *ConnectHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect, mqtt.OnDisconnect,
	}, []byte{b})
}

// OnConnect 在建立连接时候 将ip与uuid用内联client发送到指定主题
func (h *ConnectHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	// 发送消息到指定主题
	topic := h.connectTopic
	message := cl.Net.Remote
	//将clientID 和ip 组成json对象发送
	message = "{\"uuid\":\"" + cl.ID + "\",\"ip\":\"" + message + "\"}"
	h.Log.Debug("send message", slog.String("topic", topic), slog.String("message", message))
	go func() {
		h.server.Publish(topic, []byte(message), false, 0)
	}()
	return nil
}

// OnDisConnect 在断开连接时候 将uuid用内联client发送到指定主题
func (h *ConnectHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	// 发送消息到指定主题
	topic := h.disConnectTopic
	//将clientID 组成json对象发送
	message := "{\"uuid\":\"" + cl.ID + "\"}"
	h.Log.Debug("send message", slog.String("topic", topic), slog.String("message", message))
	go func() {
		h.server.Publish(topic, []byte(message), false, 0)
	}()
}
