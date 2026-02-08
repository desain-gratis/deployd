package deployjob

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/desain-gratis/deployd/src/entity"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
)

type httpHandler struct {
	state        *state
	dependencies *Dependencies
}

func (h *httpHandler) SubmitJob(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	_ = r.Header.Get("X-Namespace") //for tradition

	ctx := r.Context()

	limitR := http.MaxBytesReader(w, r.Body, 100000000)
	payload, err := io.ReadAll(limitR)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	var dj entity.SubmitDeploymentJobRequest
	err = json.Unmarshal(payload, &dj)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data"}`) // TODO: more appropriate
		return
	}

	if dj.Ns == "" {
		fmt.Fprintf(w, `{"error": "Namespace is required"}`) // TODO: more appropriate
		return
	}

	if dj.Service.Id == "" {
		fmt.Fprintf(w, `{"error": "service id is required. the rest of service configuration can be left empty"}`) // TODO: more appropriate
		return
	}

	// check if valid service
	services, err := h.dependencies.ServiceDefinitionUsecase.Get(ctx, dj.Ns, nil, dj.Service.Id)
	if err != nil {
		fmt.Fprintf(w, `{"error": "error get service definition: %v"}`, err) // TODO: more appropriate
		return
	}
	service := services[0]

	// check the latest job for this service
	// we can get the latest as long as the base storage uses "Incremental ID" type
	jobs, err := h.dependencies.JobUsecase.Get(ctx, dj.Ns, []string{dj.Service.Id}, "")
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to parse data: %v"}`, err) // TODO: more appropriate
		return
	}

	if len(jobs) > 0 {
		// extra validation for > 0 job
		latestJob := jobs[0]

		// if too soon, possible duplicate!
		if time.Since(latestJob.PublishedAt) < time.Duration(10*time.Second) {
			fmt.Fprintf(w, `{"error": "too fast, please wait"}`) // TODO: more appropriate
			return
		}
	}

	dj.Service = *service
	dj.PublishedAt = time.Now()

	modifySecret := "generate secret"
	dj.ModifyKey = &modifySecret // TODO: nice to have; only user that have the secret can update this state
	// or authorized at higher level (eg. based on namespace); but this one of the basic tool we can use

	result, err := h.dependencies.RaftJobUsecase.SubmitJob(ctx, dj)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to submit job: %v"}`, err) // TODO: more appropriate
		return
	}

	fmt.Fprintf(w, `{"success": "job submitted with id: %v"}`, result.Id)
}

func (h *httpHandler) CancelJob(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	service := p.ByName("service")
	jobID := p.ByName("id")
	ns := r.Header.Get("X-Namespace")
	secret := r.Header.Get("X-Modify-Secret")

	ctx := r.Context()
	// check the latest job for this service
	// we can get the latest as long as the base storage uses "Incremental ID" type
	jobs, err := h.dependencies.JobUsecase.Get(ctx, ns, []string{service}, jobID)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to get existing job: %v"}`, err) // TODO: more appropriate
		return
	}
	job := jobs[0]
	// latestJobMeta := jobs[0].Meta

	if *job.Request.ModifyKey != secret {
		// cihuuy
		log.Warn().Msgf("who are you editing me! but I'll allow it for now :)")
	}

	// if latestJob.Request.Service.

	_, err = h.dependencies.RaftJobUsecase.CancelJob(ctx, entity.CancelJobRequest{
		Ns:      ns,
		Id:      jobID,
		Service: service,
	})
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to submit job: %v"}`, err) // TODO: more appropriate
		return
	}

	// we can make it sync by subscribing to the topic.. but can be done later..

	fmt.Fprintf(w, `{"error": "cancelling job.."}`)
}

func (h *httpHandler) ConfirmDeployment(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "deployment confirmed")
}
