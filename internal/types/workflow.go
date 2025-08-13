package types

type WorkflowTriggerType string
type WorkflowActionType string

const (
	TriggerIGCommentReceived WorkflowTriggerType = "IG_COMMENT_RECEIVED"
	ActionIGSendMsg          WorkflowActionType  = "IG_SEND_MSG"
)

type SafetyCombinedLimits struct {
	MaxActionsPerHour   int    `json:"maxActionsPerHour"`
	MaxActionsPerDay    int    `json:"maxActionsPerDay"`
	DelayBetweenActions [2]int `json:"delayBetweenActions"`
	CommentToDmDelay    [2]int `json:"commentToDmDelay"`
}

type SafetyActionTypes struct {
	EnableCommentReply bool `json:"enableCommentReply"`
	EnableDMReply      bool `json:"enableDMReply"`
}

type SafetyContentRules struct {
	MaxMentions int `json:"maxMentions"`
	MaxHashtags int `json:"maxHashtags"`
}

type SafetyConfig struct {
	Enabled        bool                 `json:"enabled"`
	Mode           string               `json:"mode"`
	CombinedLimits SafetyCombinedLimits `json:"combinedLimits"`
	ActionTypes    SafetyActionTypes    `json:"actionTypes"`
	ContentRules   SafetyContentRules   `json:"contentRules"`
}

type IGUserCommentData struct {
	SelectedPostID  []string `json:"selectedPostId"`
	IncludeKeywords []string `json:"includeKeywords"`
	ExcludeKeywords []string `json:"excludeKeywords"`
}

type IGReplyData struct {
	PublicReplies []string      `json:"publicReplies"`
	DMMessage     string        `json:"dmMessage"`
	Buttons       []ReplyButton `json:"buttons"`
	Safety        SafetyConfig  `json:"safetyConfig"`
}

type ReplyButton struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type Node struct {
	ID   string                 `json:"id"`
	Type string                 `json:"types"`
	Data map[string]interface{} `json:"data"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type WorkflowDefinition struct {
	ID    string `json:"id"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type WorkflowRepo interface {
	// Ambil workflow aktif yang punya trigger IG_COMMENT_RECEIVED untuk brand/account tertentu
	ListActiveWorkflowsForIGAccount(igBusinessID string) ([]*WorkflowDefinition, error)
}
