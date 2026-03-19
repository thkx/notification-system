package model

// Notification 通知结构，定义了通知的基本信息
type Notification struct {
	ID        string   // 通知ID
	UserID    string   // 用户ID
	Type      string   // 通知类型
	Content   string   // 通知内容
	Channels  []string // 通知渠道列表
	Priority  int      // 通知优先级
	Scheduled string   // 调度时间
}
