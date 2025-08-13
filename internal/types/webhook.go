package types

// IG webhook envelope (disederhanakan)
type IGWebhookEnvelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				CommentID string `json:"id"`
				PostID    string `json:"post_id"`
				Text      string `json:"text"`
				From      struct {
					ID       string `json:"id"`
					Username string `json:"username"`
				} `json:"from"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}
