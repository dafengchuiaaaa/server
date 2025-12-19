package hooks

import (
	"encoding/json"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// DeduplicationHook 用于过滤重复消息
// 本钩子在消息接收时（OnPublish阶段）检查并过滤重复消息
// 如果发现重复消息，会直接返回 packets.ErrRejectPacket，这样消息会被直接丢弃
// 这种处理发生在消息转发给订阅者之前，确保重复消息不会被处理或存储
type DeduplicationHook struct {
	mqtt.HookBase
	mu sync.RWMutex

	// 消息缓存，按 UUID 分类
	// key: UUID, value: 最后一条消息的时间戳
	msgCache map[string]int64

	// 配置选项
	targetTopic    string        // 目标主题
	timestampField string        // 时间戳字段
	uuidField      string        // UUID 字段
	countField     string        // count 字段
	timeWindow     int64         // 时间窗口（秒）
	cleanInterval  time.Duration // 清理间隔
}

// NewDeduplicationHook 创建一个新的去重钩子
func NewDeduplicationHook() *DeduplicationHook {
	h := &DeduplicationHook{
		msgCache:       make(map[string]int64),
		targetTopic:    "device/contact", // 目标主题
		timestampField: "timestamp",      // 时间戳字段
		uuidField:      "uuid",           // UUID 字段
		countField:     "count",          // count 字段
		timeWindow:     20,               // 20秒内视为重复
		cleanInterval:  5 * time.Minute,  // 5分钟清理一次缓存
	}

	// 启动定期清理过期缓存的任务
	go h.startCleanupTask()

	return h
}

// ID 返回 Hook 的 ID
func (h *DeduplicationHook) ID() string {
	return "deduplication"
}

// Provides 返回 Hook 提供的功能
func (h *DeduplicationHook) Provides(b byte) bool {
	return b == mqtt.OnPublish // 使用 OnPublish 钩子在消息收到后立即过滤
}

// OnPublish 在收到消息时检查是否需要过滤
// 此方法在消息被转发给订阅者之前执行
// 如果返回 packets.ErrRejectPacket，消息会被直接丢弃
func (h *DeduplicationHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// 只处理目标主题
	if pk.TopicName != h.targetTopic {
		return pk, nil
	}

	// 解析消息 JSON
	var msgData map[string]interface{}
	if err := json.Unmarshal(pk.Payload, &msgData); err != nil {
		// JSON 解析失败，不过滤
		h.Log.Debug("消息解析失败", "error", err)
		return pk, nil
	}

	// 提取 UUID
	uuid, ok := h.extractString(msgData, h.uuidField)
	if !ok {
		// 找不到 UUID，不过滤
		h.Log.Debug("消息缺少 UUID 字段")
		return pk, nil
	}

	// 提取 count，如果为 0 表示客户端刚启动，直接放行
	if count, exists := h.extractInt(msgData, h.countField); exists && count == 0 {
		h.Log.Debug("客户端启动消息，直接放行", "uuid", uuid)
		// 重置该 UUID 的缓存时间
		h.mu.Lock()
		h.msgCache[uuid] = time.Now().Unix()
		h.mu.Unlock()
		return pk, nil
	}

	serverTime := time.Now().Unix()

	// 检查是否为重复消息
	h.mu.Lock()
	defer h.mu.Unlock()

	if lastTs, exists := h.msgCache[uuid]; exists {
		// 计算时间差（秒）
		timeDiff := serverTime - lastTs

		// 如果时间差在窗口内，且新消息时间戳大于等于旧消息，视为重复
		if timeDiff >= 0 && timeDiff <= h.timeWindow {
			h.Log.Debug("过滤重复消息", "uuid", uuid, "time_diff", timeDiff)
			// 更新时间戳为最新的
			// h.msgCache[uuid] = serverTime
			return pk, packets.ErrRejectPacket // 拒绝此消息
		}
	}
	// 不是重复消息，更新缓存
	h.msgCache[uuid] = serverTime
	return pk, nil
}

// 从消息中提取字符串字段
func (h *DeduplicationHook) extractString(data map[string]interface{}, field string) (string, bool) {
	value, ok := data[field]
	if !ok {
		return "", false
	}

	strValue, ok := value.(string)
	if !ok {
		return "", false
	}

	return strValue, true
}

// 从消息中提取 int 字段，区分 0 和不存在
func (h *DeduplicationHook) extractInt(data map[string]interface{}, field string) (int64, bool) {
	value, ok := data[field]
	if !ok {
		return 0, false // 字段不存在
	}

	// JSON 解析数字默认为 float64
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

// 清理过期缓存
func (h *DeduplicationHook) startCleanupTask() {
	ticker := time.NewTicker(h.cleanInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.cleanExpiredCache()
	}
}

// 清理过期缓存
func (h *DeduplicationHook) cleanExpiredCache() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 当前时间戳
	now := time.Now().Unix()

	// 删除超过 1 小时未更新的缓存
	expireThreshold := now - 3600

	for uuid, timestamp := range h.msgCache {
		if timestamp < expireThreshold {
			delete(h.msgCache, uuid)
		}
	}

	h.Log.Debug("清理过期缓存完成", "cache_size", len(h.msgCache))
}

// GetStats 获取统计数据
func (h *DeduplicationHook) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":   len(h.msgCache),
		"target_topic": h.targetTopic,
		"time_window":  h.timeWindow,
	}
}

// SetConfig 配置去重参数
func (h *DeduplicationHook) SetConfig(topic, uuidField, timestampField string, timeWindow int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.targetTopic = topic
	h.uuidField = uuidField
	h.timestampField = timestampField
	h.timeWindow = timeWindow
}
