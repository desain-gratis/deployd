package configurenginxunit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// HTTP interface of the deployment job
// This one is an interface to raft as well to coordinate the job
type httpHandler struct {
	localWorker  *localWorker
	dependencies *Dependencies
}

func (h *httpHandler) SubmitNginxUnitConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_ = r.Header.Get("X-Namespace") //for tradition

	ctx := r.Context()

	limitR := http.MaxBytesReader(w, r.Body, 100000000)
	payload, err := io.ReadAll(limitR)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	validJson := make(map[string]any)
	err = json.Unmarshal(payload, &validJson)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	result, err := h.dependencies.RaftNginxUnitUsecase.UpdateNginxUnitConfig(ctx, payload)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to submit job: %v"}`, err) // TODO: more appropriate
		return
	}

	resp, _ := json.Marshal(map[string]any{
		"success": result,
	})

	fmt.Fprintln(w, string(resp))
}
