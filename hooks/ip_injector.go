package hooks

import (
	"bytes"
	"encoding/json"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// IPInjectorHook 用于在消息中注入 IP 地址
type IPInjectorHook struct {
	targetTopic []string
	mqtt.HookBase
}

// NewIPInjectorHook 创建一个新的 IP 注入器钩子
func NewIPInjectorHook() *IPInjectorHook {
	return &IPInjectorHook{
		targetTopic: []string{"device/contact", "device/report/restart"},
	}
}

// ID 返回 Hook 的 ID
func (h *IPInjectorHook) ID() string {
	return "ip-injector"
}

// Provides 返回 Hook 提供的功能
func (h *IPInjectorHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPublish,
	}, []byte{b})
}

func (h *IPInjectorHook) isTargetTopic(topic string) bool {
	for _, t := range h.targetTopic {
		if t == topic {
			return true
		}
	}
	return false
}

// OnPublish 在消息发布时注入 IP 地址
func (h *IPInjectorHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if !h.isTargetTopic(pk.TopicName) {
		return pk, nil
	}
	// 构建包含元数据的新 payload
	newPayload := struct {
		Meta struct {
			IP string `json:"ip"`
		} `json:"meta"`
		Data json.RawMessage `json:"data"`
	}{
		Meta: struct {
			IP string `json:"ip"`
		}{
			IP: cl.Net.Remote,
		},
		Data: pk.Payload, // 保留原始 payload
	}

	// 序列化为 JSON
	payloadBytes, err := json.Marshal(newPayload)
	if err != nil {
		return pk, err
	}

	pk.Payload = payloadBytes
	return pk, nil
}
