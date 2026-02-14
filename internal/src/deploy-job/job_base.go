package deployjob

import "errors"

var _ Job = &jobBase{}

// convenient base struct that implement Job interface to be composed from by other job
type jobBase struct {
	Name         string  `json:"name"`
	Status       Status  `json:"status"`
	RetryCount   uint8   `json:"retry_count"`
	CurrentStep  uint8   `json:"current_step"`
	TotalSteps   uint8   `json:"total_step"`
	Url          string  `json:"url"`
	ErrorMessage *string `json:"error_message"`
}

func (c *jobBase) GetName() string {
	return c.Name
}

func (c *jobBase) GetRetryCount() uint8 {
	return c.RetryCount
}

func (c *jobBase) GetStatus() Status {
	// external status can be different than internal one;
	// in this implementation, the internal state is the same as the common external ones

	// implementation should map their internal status to this common one

	return c.Status
}

func (c *jobBase) GetDAG() DAG {
	// Hardcoded, no need to be generic here
	return DAG{
		Vertices: make([]Job, 0),
		Edges:    make([]uint8, 0),
	}
}

func (c *jobBase) GetCurrentSteps() uint8 {
	return c.CurrentStep
}

func (c *jobBase) GetTotalSteps() uint8 {
	return c.TotalSteps
}

func (c *jobBase) GetURL() string {
	return c.Url
}

// There is no main execute;
func (c *jobBase) Execute() error {
	return errors.New("not implemented")
}

func (c *jobBase) SetErrorMessage(msg string) {
	c.ErrorMessage = &msg
}

func (c *jobBase) GetErrorMessage() string {
	if c.ErrorMessage == nil {
		return ""
	}
	return *c.ErrorMessage
}
