package configurewebsite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/desain-gratis/deployd/src/entity"
	"github.com/julienschmidt/httprouter"
)

// HTTP interface of the deployment job
// This one is an interface to raft as well to coordinate the job
type httpHandler struct {
	localWorker  *localWorker
	dependencies *Dependencies
}

func (h *httpHandler) ConfigureWebsite(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_ = r.Header.Get("X-Namespace") //for tradition

	ctx := r.Context()

	limitR := http.MaxBytesReader(w, r.Body, 100000000)
	payload, err := io.ReadAll(limitR)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	var req entity.ConfigureWebsiteRequest
	err = json.Unmarshal(payload, &req)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	allHosts, err := h.dependencies.HostConfigUsecase.Get(ctx, "deployd", nil, "")
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to get hosts"}`) // TODO: more appropriate
		return
	}
	hostByName := make(map[string]*entity.Host)
	for _, host := range allHosts {
		hostByName[host.Host] = host
	}

	req.TargetHosts = make([]string, 0)
	for _, host := range allHosts {
		req.TargetHosts = append(req.TargetHosts, host.Host)
	}

	req.Time = time.Now()

	result, err := h.dependencies.RaftConfigureWebsite.ConfigureWebsite(ctx, req)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to submit job: %v"}`, err) // TODO: more appropriate
		return
	}

	resp, _ := json.Marshal(map[string]any{
		"success": result,
	})

	fmt.Fprintln(w, string(resp))
}
