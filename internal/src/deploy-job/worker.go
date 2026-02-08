package deployjob

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/desain-gratis/common/lib/notifier"
	deployjob "github.com/desain-gratis/deployd/internal/src/raft-app/deploy-job"
	"github.com/desain-gratis/deployd/src/entity"
	"github.com/rs/zerolog/log"
)

// shared state for integration
type activeJob struct {
	dependencies *Dependencies
	ctx          context.Context
	cancel       context.CancelFunc
	status       string // job status (i prefer different than job status itself)
	job          entity.DeploymentJob
	topic        notifier.Topic
	log          *slog.Logger
	host         *entity.Host
}

type state struct {
	host *entity.Host

	// configuration job
	activeJobs map[string]*activeJob

	// there is maybe other job

	// TODO: put DAG / graph definition here
}

type worker struct {
	state        *state
	dependencies *Dependencies
	host         *entity.Host
}

func (w *worker) StartConsumer(topic notifier.Topic, subscription notifier.Subscription) {
	go func() {
		for event := range subscription.Listen() {
			switch value := event.(type) {
			case deployjob.EventDeploymentJobCreated:
				w.configureHost(topic, value.Job) // test no goroutine
			case deployjob.EventDeploymentJobCancelled:
				w.cancelActiveJob(topic, value.Job)
			default:
			}
		}
	}()
}

func (w *worker) cancelActiveJob(out notifier.Topic, job entity.DeploymentJob) {
	// todo: prepare locking
	activeJob, ok := w.state.activeJobs[getKey(job)]
	if !ok {
		return
	}

	activeJob.cancel()
}

func (w *worker) configureHost(out notifier.Topic, job entity.DeploymentJob) {
	// todo: prepare locking
	if _, ok := w.state.activeJobs[getKey(job)]; ok {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	pr, pw := io.Pipe()
	multi := io.MultiWriter(os.Stdout, pw)

	logger := slog.New(slog.NewJSONHandler(multi, &slog.HandlerOptions{})).
		With(
			"host", w.state.host.Host,
			"namespace", job.Request.Ns,
			"service", job.Request.Service,
			"id", job.Id,
			"secret_version", job.Request.SecretVersion,
			"env_version", job.Request.EnvVersion,
			"build_version", job.Request.BuildVersion,
		)

	activeJob := &activeJob{
		ctx:          ctx,
		cancel:       cancel,
		status:       "started",
		job:          job,
		topic:        out,
		log:          logger,
		host:         w.host,
		dependencies: w.dependencies,
	}
	// TODO: forward slog output to topic

	w.state.activeJobs[getKey(job)] = activeJob

	scanner := bufio.NewScanner(pr)

	// Todo move in their own module
	go func() {
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()
			out.Broadcast(ctx, line) // broadcast log, or maybe make it wrapped
		}

		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()

	err := w.dependencies.RaftJobUsecase.FeedHostConfigurationUpdate(ctx, deployjob.ConfigurationUpdateRequest{
		Ns:       job.Ns,
		Id:       job.Id,
		Service:  job.Request.Service.Id,
		HostName: w.state.host.Host,
		Status:   entity.DeploymentJobStatusConfiguring,
		Message:  "Configurating & Installing",
	})
	if err != nil {
		activeJob.status = "failed"
		log.Err(err).Msgf("failed to update job configuration status %v", err)
		return
	}

	defer func() {
		activeJob.log.Info("configuring host finished", "status", activeJob.status)
	}()

	go func() {
		defer pw.Close()

		activeJob.configureUbuntu(ctx)

		updateStatus := entity.DeploymentJobStatusFailed
		switch activeJob.status {
		case "success":
			updateStatus = entity.DeploymentJobStatusConfigured
		case "failed":
			updateStatus = entity.DeploymentJobStatusFailed
		case "cancelled":
			updateStatus = entity.DeploymentJobStatusCancelled
		}

		err = w.dependencies.RaftJobUsecase.FeedDeploymentUpdate(ctx, deployjob.DeploymentUpdateRequest{
			Ns:       job.Ns,
			Id:       job.Id,
			Service:  job.Request.Service.Id,
			HostName: w.state.host.Host,
			Status:   updateStatus,
		})
		if err != nil {
			log.Err(err).Msgf("failed to update deployment job status %v", err)
			return
		}
	}()
}

func (a *activeJob) configureUbuntu(ctx context.Context) {

	a.log.Info("configuring host directory")
	// for i := range 100 {
	// 	log.Info().Msgf("GGWP %v", i)
	// }
	if err := ctx.Err(); err != nil {
		log.Info().Msgf("KOQ CANCELELD?")
		a.status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	basePath := fmt.Sprintf("/opt/%v_%v", a.job.Request.Ns, a.job.Request.Service.Id)

	a.log.Info("ensuring path", "path", basePath)
	err := ensureDir(basePath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring directory in base path", "path", basePath)
		return
	}

	envPath := fmt.Sprintf(basePath+"/env-release/%v", a.job.Request.EnvVersion)
	a.log.Info("ensuring path", "path", envPath)
	err = ensureDir(envPath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring env path", "path", envPath, "error", err)
		return
	}

	etcPath := fmt.Sprintf("/etc/%v_%v", a.job.Request.Ns, a.job.Request.Service.Id)
	a.log.Info("ensuring path", "path", etcPath)
	err = ensureDir(etcPath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring etc path", "path", etcPath, "error", err)
		return
	}

	tmpPath := fmt.Sprintf("/tmp/%s_%s/artifact/%v", a.job.Request.Ns, a.job.Request.Service.Id, a.job.Request.BuildVersion)
	a.log.Info("ensuring path", "tmp", tmpPath)
	err = ensureDir(tmpPath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring tmp path", "path", tmpPath, "error", err)
		return
	}

	systemdPath := "/etc/systemd/system"
	a.log.Info("ensuring path", "path", systemdPath)
	err = ensureDir(systemdPath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err)
		return
	}

	// write systemd
	a.log.Info("writing unit file")
	if err := ctx.Err(); err != nil {
		a.status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		content := BuildUnit(a.job.Request.Ns, a.job.Request.Service.Id, a.job.Request.Service.Description, a.job.Request.Service.ExecutablePath)
		name := fmt.Sprintf("%v_%v.service", a.job.Request.Ns, a.job.Request.Service.Id)
		tmp := filepath.Join(systemdPath, name+".tmp")
		final := filepath.Join(systemdPath, name)
		if err1 := os.WriteFile(tmp, []byte(content), 0644); err1 != nil {
			a.status = "failed"
			a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err1)
			return err1
		}
		err1 := os.Rename(tmp, final)
		if err1 != nil {
			a.status = "failed"
			a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err1)
			return err1
		}

		return nil
	}()
	if err != nil {
		return
	}

	// start more heavier operation
	a.log.Info("downloading .env")
	if err := ctx.Err(); err != nil {
		a.status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		envData, err1 := a.dependencies.EnvUsecase.Get(ctx, a.job.Request.Ns, []string{a.job.Request.Service.Id}, strconv.FormatUint(a.job.Request.EnvVersion, 10))
		if err1 != nil || len(envData) == 0 {
			a.status = "failed"
			a.log.Error("error while downloading env", "error", err1)
			return err1
		}

		env := envData[0]

		tmpEnv := make([]string, 0, len(env.Value))
		for k, v := range env.Value {
			tmpEnv = append(tmpEnv, fmt.Sprintf("%v=%v", strings.ToUpper(k), strconv.Quote(v)))
		}

		sort.Slice(tmpEnv, func(i, j int) bool {
			return strings.Compare(tmpEnv[i], tmpEnv[j]) < 0
		})

		a.log.Info("writing .env")

		path := envPath + "/overwrite.env"

		f, err1 := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err1 != nil {
			a.status = "failed"
			a.log.Error("error while opening env file", "path", path, "error", err1)
			return err1
		}
		defer f.Close()

		for _, env := range tmpEnv {
			fmt.Fprintln(f, env)
		}

		return nil
	}()
	if err != nil {
		return
	}

	buildReleasePath := fmt.Sprintf(basePath+"/build-release/%v", a.job.Request.BuildVersion)
	err = ensureDir(buildReleasePath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring build release path", "path", buildReleasePath, "error", err)
		return
	}

	// TODO: use per file based check / more robust approach;
	isBuildEmpty, err := isEmptyDir(buildReleasePath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while check existing installation inside", "path", buildReleasePath, "error", err)
		return
	}

	// TODO: remove this; after finding a way to optimize use installation
	if !isBuildEmpty {
		a.status = "success"
		a.log.Info("host is configured")
		return
	}

	a.log.Info("downloading build artifact")
	if err := ctx.Err(); err != nil {
		a.status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		buildId := strconv.FormatUint(a.job.Request.BuildVersion, 10)
		buildArtifact, _, err1 := a.dependencies.BuildArtifactUsecase.GetAttachment(
			ctx,
			a.job.Request.Ns,
			[]string{a.job.Request.Service.Id, buildId},
			fmt.Sprintf("%v/%v", a.host.OS, a.host.Architecture), // attachment can have one to many, so we're restricting to one
		)
		if err1 != nil {
			a.status = "failed"
			a.log.Error("error while getting build artifact", "error", err1)
			return err1
		}
		defer buildArtifact.Close()

		f, err1 := os.OpenFile(tmpPath+"/release.tar.gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err1 != nil {
			a.status = "failed"
			a.log.Error("error while opening env file", "error", err1)
			return err1
		}
		defer f.Close()

		// Download
		err1 = Copy(ctx, f, buildArtifact)
		if err1 != nil {
			a.status = "failed"
			a.log.Error("error while writing artifact file", "error", err1)
			return err1
		}

		return nil
	}()
	if err != nil {
		return
	}

	a.log.Info("extracting build artifact")

	tmp := buildReleasePath + ".tmp"
	err = os.RemoveAll(tmp)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while removing old artifact", "error", err)
		return
	}

	err = ensureDir(tmp)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while ensuring extracted artifact dir", "error", err)
		return
	}

	err = ExtractTarGzStrip(ctx, tmpPath+"/release.tar.gz", tmp)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while extracting artifact file", "error", err)
		return
	}

	err = os.RemoveAll(buildReleasePath) // delete previous
	if err != nil {
		a.status = "failed"
		a.log.Error("error while deleting previous installation", "error", err)
		return
	}

	err = os.Rename(tmp, buildReleasePath)
	if err != nil {
		a.status = "failed"
		a.log.Error("error while renaming artifact file", "error", err)
		return
	}

	a.status = "success"
	a.log.Info("successfully configured host")
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func isEmptyDir(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil // empty
	}
	if err != nil {
		return false, err
	}

	return false, nil // has at least one entry
}

func getKey(job entity.DeploymentJob) string {
	keys := []string{job.Ns, job.Request.Service.Id, job.Id}
	return strings.Join(keys, "\\")
}
