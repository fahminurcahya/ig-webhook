package queue

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueDefault  = "default"
	QueuePriority = "priority"

	TypeSendPublicReply = "ig:send_public_reply"
	TypeSendDM          = "ig:send_dm"
)

type TaskSendPublicReplyPayload struct {
	BrandID    string
	CommentID  string
	Message    string
	IGToken    string
	WorkflowID string
	NodeID     string
}

type TaskSendDMPayload struct {
	BrandID           string
	RecipientIGUserID string
	Message           string
	IGToken           string
	WorkflowID        string
	NodeID            string
}

func RandDelaySec(min, max int) time.Duration {
	if max <= min {
		return time.Duration(min) * time.Second
	}
	d := rand.Intn(max-min+1) + min
	return time.Duration(d) * time.Second
}

func NewPublicReplyTask(p TaskSendPublicReplyPayload, delay time.Duration) (*asynq.Task, []asynq.Option) {
	b, _ := json.Marshal(p)
	t := asynq.NewTask(TypeSendPublicReply, b, asynq.Queue(QueueDefault))
	opts := []asynq.Option{
		asynq.MaxRetry(8),
		asynq.ProcessIn(delay),
		asynq.Timeout(15 * time.Second),
	}
	return t, opts
}

func NewDMTask(p TaskSendDMPayload, delay time.Duration) (*asynq.Task, []asynq.Option) {
	b, _ := json.Marshal(p)
	t := asynq.NewTask(TypeSendDM, b, asynq.Queue(QueueDefault))
	opts := []asynq.Option{
		asynq.MaxRetry(8),
		asynq.ProcessIn(delay),
		asynq.Timeout(15 * time.Second),
	}
	return t, opts
}
