package realtime

type inboundMessage struct {
	ConversationID  string `json:"conversationId"`
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

type publishMessage struct {
	inboundMessage
	SenderID string `json:"senderId"`
}

type outboundMessage struct {
	ID              string `json:"id"`
	ConversationID  string `json:"conversationId"`
	SenderID        string `json:"senderId"`
	MessageSequence int64  `json:"messageSequence"`
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}
