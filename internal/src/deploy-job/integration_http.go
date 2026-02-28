package deployjob

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"

	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
)

// HTTP interface of the deployment job
// This one is an interface to raft as well to coordinate the job
type httpHandler struct {
	localWorker  *localWorker
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
		if time.Since(latestJob.PublishedAt) < time.Duration(5*time.Second) {
			fmt.Fprintf(w, `{"error": "too fast, please wait"}`) // TODO: more appropriate
			return
		}
	}

	// overwrite host data with latest data from host config to snapshot;
	// maybe use optimistic lock in the request (later)
	allHosts, err := h.dependencies.HostConfigUsecase.Get(ctx, dj.Ns, nil, "")
	if err != nil {
		fmt.Fprintf(w, `{"error": "error during getting current hosts: %v"}`, err) // TODO: more appropriate
		return
	}

	hostByName := make(map[string]*entity.Host)
	for _, host := range allHosts {
		hostByName[host.Host] = host
	}
	for idx, target := range dj.TargetHosts {
		fullHostData, ok := hostByName[target.Host]
		if !ok {
			fmt.Fprintf(w, `{"error": "host not found %v"}`, target.Host) // TODO: more appropriate
			return
		}
		dj.TargetHosts[idx] = *fullHostData
	}

	dj.Service = *service
	dj.PublishedAt = time.Now()

	dj.RaftPort = RandomPort()

	modifySecret := "generate secret"
	dj.ModifyKey = &modifySecret // TODO: nice to have; only user that have the secret can update this state
	// or authorized at higher level (eg. based on namespace); but this one of the basic tool we can use

	result, err := h.dependencies.RaftJobUsecase.SubmitJob(ctx, dj)
	if err != nil {
		fmt.Fprintf(w, `{"error": "failed to submit job: %v"}`, err) // TODO: more appropriate
		return
	}

	// TODO: handle error based on status
	if result.SubmitJobStatus == deployjob.SubmitJobStatusNeedRetry {
		// TODO: need retry!!!
	}

	resp, _ := json.Marshal(map[string]any{
		"success": result,
	})

	fmt.Fprintln(w, string(resp))
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
	// TODO:
	fmt.Fprintf(w, "deployment confirmed")
}

func (h *httpHandler) StreamLog(topic notifier.Topic) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		jobParam := p.ByName("active-job")
		_, ok := h.localWorker.deploymentJobPool[jobParam]
		if !ok {
			// todo: 404 not found
			fmt.Fprintf(w, `{"error": "active job not found"}`)
			return
		}

		// job

	}
}

func RandomPort() uint16 {
	return uint16(rand.Intn(65535-10000) + 10000)
}
