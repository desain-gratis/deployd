package deployjob

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusRetrying   Status = "retrying"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
	StatusSuccess    Status = "success"
	StatusInvalid    Status = "invalid"
)

// Trying to make job generic
// Not specific to this usecase, TODO: move to common library
// All prefixed with Get.. to make it easy for each implementation expose their state with the name without prefix
type Job interface {
	GetName() string
	GetRetryCount() uint8
	GetStatus() Status
	GetDAG() DAG
	GetCurrentSteps() uint8
	GetTotalSteps() uint8
	GetURL() string

	Execute() error // intentional no return value; it should be a side effect that represented by JobStatus
}

type DAG struct {
	// Vertices contains job
	Vertices []Job `json:"vertices"`

	// Edges contain vertices index; always len(Edges) == len(Vertices)*2
	Edges []uint8 `json:"edges"`
}
