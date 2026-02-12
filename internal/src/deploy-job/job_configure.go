package deployjob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/desain-gratis/deployd/src/entity"
)

var _ Job = &configureJob{}

// shared state for integration
// represents an in-memory job / process inside a host.
type configureJob struct {
	dependencies *Dependencies
	ctx          context.Context
	cancel       context.CancelFunc
	topic        notifier.Topic
	log          *slog.Logger
	host         *entity.Host

	Name        string `json:"name"`
	Status      Status `json:"status"`
	RetryCount  uint8  `json:"retry_count"`
	CurrentStep uint8  `json:"current_step"`

	// job definition. Maybe can take only the important fields..
	// eg. only job ID, namespace, etc..
	// but lets keep them all for now
	Job entity.DeploymentJob `json:"job"`
}

func (c *configureJob) GetName() string {
	return c.Name
}

func (c *configureJob) GetRetryCount() uint8 {
	return c.RetryCount
}

func (c *configureJob) GetStatus() Status {
	// external status can be different than internal one;
	// in this implementation, the internal state is the same as the common external ones
	return c.Status
}

func (c *configureJob) GetDAG() DAG {
	// Hardcoded, no need to be generic here
	return DAG{
		Vertices: make([]Job, 0),
		Edges:    make([]uint8, 2),
	}
}

func (c *configureJob) GetCurrentSteps() uint8 {
	return c.CurrentStep
}

func (c *configureJob) GetTotalSteps() uint8 {
	// Hardcoded, no need to be generic here
	return 10
}

func (c *configureJob) Execute(ctx context.Context) {
	c.configureUbuntu(ctx)
}

// TODO: separate it into their own module later...
func (a *configureJob) configureUbuntu(ctx context.Context) {
	a.log.Info("configuring host directory")

	if err := ctx.Err(); err != nil {
		a.Status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	basePath := fmt.Sprintf("/opt/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)

	a.log.Info("ensuring path", "path", basePath)
	err := ensureDir(basePath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring directory in base path", "path", basePath, "error", err)
		return
	}

	envPath := fmt.Sprintf(basePath+"/env-release/%v", a.Job.Request.EnvVersion)
	a.log.Info("ensuring path", "path", envPath)
	err = ensureDir(envPath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring env path", "path", envPath, "error", err)
		return
	}

	etcPath := fmt.Sprintf("/etc/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)
	a.log.Info("ensuring path", "path", etcPath)
	err = ensureDir(etcPath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring etc path", "path", etcPath, "error", err)
		return
	}

	tmpPath := fmt.Sprintf("/tmp/%s_%s/artifact/%v", a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.BuildVersion)
	a.log.Info("ensuring path", "tmp", tmpPath)
	err = ensureDir(tmpPath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring tmp path", "path", tmpPath, "error", err)
		return
	}

	systemdPath := "/etc/systemd/system"
	a.log.Info("ensuring path", "path", systemdPath)
	err = ensureDir(systemdPath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err)
		return
	}

	// write systemd
	a.log.Info("writing unit file")
	if err := ctx.Err(); err != nil {
		a.Status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		content := BuildUnit(a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.Service.Description, a.Job.Request.Service.ExecutablePath)
		name := fmt.Sprintf("%v_%v.service", a.Job.Request.Ns, a.Job.Request.Service.Id)
		tmp := filepath.Join(systemdPath, name+".tmp")
		final := filepath.Join(systemdPath, name)
		if err1 := os.WriteFile(tmp, []byte(content), 0644); err1 != nil {
			a.Status = "failed"
			a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err1)
			return err1
		}
		err1 := os.Rename(tmp, final)
		if err1 != nil {
			a.Status = "failed"
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
		a.Status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		envData, err1 := a.dependencies.EnvUsecase.Get(ctx, a.Job.Request.Ns, []string{a.Job.Request.Service.Id}, strconv.FormatUint(a.Job.Request.EnvVersion, 10))
		if err1 != nil || len(envData) == 0 {
			a.Status = "failed"
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
			a.Status = "failed"
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

	buildReleasePath := fmt.Sprintf(basePath+"/build-release/%v", a.Job.Request.BuildVersion)
	err = ensureDir(buildReleasePath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring build release path", "path", buildReleasePath, "error", err)
		return
	}

	// TODO: use per file based check / more robust approach;
	isBuildEmpty, err := isEmptyDir(buildReleasePath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while check existing installation inside", "path", buildReleasePath, "error", err)
		return
	}

	// TODO: remove this; after finding a way to optimize use installation
	if !isBuildEmpty {
		a.Status = "success"
		a.log.Info("host is configured")
		return
	}

	a.log.Info("downloading build artifact")
	if err := ctx.Err(); err != nil {
		a.Status = "cancelled"
		a.log.Error("job cancelled", "error", err)
		return
	}

	err = func() error {
		buildId := strconv.FormatUint(a.Job.Request.BuildVersion, 10)
		buildArtifact, _, err1 := a.dependencies.BuildArtifactUsecase.GetAttachment(
			ctx,
			a.Job.Request.Ns,
			[]string{a.Job.Request.Service.Id, buildId},
			fmt.Sprintf("%v/%v", a.host.OS, a.host.Architecture), // attachment can have one to many, so we're restricting to one
		)
		if err1 != nil {
			a.log.Error("error while getting build artifact", "error", err1)
			a.Status = "failed"

			if errors.Is(err1, context.Canceled) {
				a.Status = StatusCancelled
			}

			return err1
		}
		defer buildArtifact.Close()

		f, err1 := os.OpenFile(tmpPath+"/release.tar.gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err1 != nil {
			a.Status = "failed"
			a.log.Error("error while opening env file", "error", err1)
			return err1
		}
		defer f.Close()

		// Download
		err1 = Copy(ctx, f, buildArtifact)
		if err1 != nil {
			a.log.Error("error while writing artifact file", "error", err1)
			a.Status = "failed"

			if errors.Is(err1, context.Canceled) {
				a.Status = StatusCancelled
			}

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
		a.Status = "failed"
		a.log.Error("error while removing old artifact", "error", err)
		return
	}

	err = ensureDir(tmp)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while ensuring extracted artifact dir", "error", err)
		return
	}

	err = ExtractTarGzStrip(ctx, tmpPath+"/release.tar.gz", tmp)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while extracting artifact file", "error", err)
		return
	}

	err = os.RemoveAll(buildReleasePath) // delete previous
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while deleting previous installation", "error", err)
		return
	}

	err = os.Rename(tmp, buildReleasePath)
	if err != nil {
		a.Status = "failed"
		a.log.Error("error while renaming artifact file", "error", err)
		return
	}

	a.Status = "success"
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
