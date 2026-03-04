package types

// HasPayload reports whether the message has non-empty Content or ContentParts.
func HasPayload(msg Message) bool {
	return msg.Content != "" || len(msg.ContentParts) > 0
}
