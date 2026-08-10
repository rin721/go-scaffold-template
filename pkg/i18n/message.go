package i18n

// Message 定义一次翻译请求。
type Message struct {
	ID             string
	Data           map[string]any
	Count          any
	DefaultMessage string
}

// LocalizeOption 用于声明一次翻译请求的可选参数。
type LocalizeOption func(*Message)

// Text 使用消息 ID 创建翻译请求。
func Text(id string, options ...LocalizeOption) Message {
	message := Message{ID: id}
	for _, option := range options {
		if option != nil {
			option(&message)
		}
	}
	return message
}

// WithData 设置模板变量。
func WithData(data map[string]any) LocalizeOption {
	return func(message *Message) {
		message.Data = data
	}
}

// WithCount 设置复数计数。
func WithCount(count any) LocalizeOption {
	return func(message *Message) {
		message.Count = count
	}
}

// WithDefault 设置消息缺失时使用的默认文案。
func WithDefault(text string) LocalizeOption {
	return func(message *Message) {
		message.DefaultMessage = text
	}
}
