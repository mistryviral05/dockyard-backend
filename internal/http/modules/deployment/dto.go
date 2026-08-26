package deployment

import "time"

type CreateDeploymentDto struct {
	DeploymentId  string
	ProjectId     string
	AgentId       string
	TriggeredBy   *string
	TriggerSource string
	Branch        string
	CommitSha     *string
	CommitMessage *string
}

type DeployRequest struct {
	Branch string `json:"branch"`
}

// DeployResponse deploy ke baad frontend ko deployment page pe bhejne ke
// liye id deta hai. Status is point pe hamesha queued hota hai, baaki
// detail GET /deployments/{id} se aati hai.
type DeployResponse struct {
	DeploymentId string `json:"deployment_id"`
	Status       string `json:"status"`
}

type DeploymentResponse struct {
	Id            string     `json:"id"`
	ProjectId     string     `json:"project_id"`
	Status        string     `json:"status"`
	TriggerSource string     `json:"trigger_source"`
	Branch        string     `json:"branch"`
	CommitSha     string     `json:"commit_sha,omitempty"`
	CommitMessage string     `json:"commit_message,omitempty"`
	ImageTag      string     `json:"image_tag,omitempty"`
	ErrorMessage  *string    `json:"error_message"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type LogResponse struct {
	Id       string    `json:"id"`
	Type     string    `json:"type"`
	Message  string    `json:"message"`
	LoggedAt time.Time `json:"logged_at"`
}

// LogPage cursor ke saath ek page deta hai. next_cursor null aane ka matlab
// hai ki abhi ke liye aur logs nahi hain. Deployment chal rahi ho to aage
// ki lines Centrifugo se aayengi, dobara poll karne ki zaroorat nahi.
type LogPage struct {
	Items      []*LogResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

func toLogResponse(l *DeploymentLog) *LogResponse {
	return &LogResponse{
		Id:       l.Id,
		Type:     l.LogType,
		Message:  l.Message.String,
		LoggedAt: l.LoggedAt,
	}
}

// toResponse keeps sql.Null* out of the JSON: those marshal as
// {"String":"","Valid":false}, which the frontend cannot use.
func toResponse(d *Deployment) *DeploymentResponse {
	res := &DeploymentResponse{
		Id:            d.Id,
		ProjectId:     d.ProjectId,
		Status:        d.Status,
		TriggerSource: d.TriggerSource.String,
		Branch:        d.Branch.String,
		CommitSha:     d.CommitSha.String,
		CommitMessage: d.CommitMessage.String,
		ImageTag:      d.ImageTag.String,
		CreatedAt:     d.CreatedAt,
	}

	if d.ErrorMessage.Valid {
		res.ErrorMessage = &d.ErrorMessage.String
	}
	if d.StartedAt.Valid {
		res.StartedAt = &d.StartedAt.Time
	}
	if d.CompletedAt.Valid {
		res.CompletedAt = &d.CompletedAt.Time
	}

	return res
}
