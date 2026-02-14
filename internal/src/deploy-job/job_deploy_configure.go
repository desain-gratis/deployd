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
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/desain-gratis/deployd/src/entity"
)

var _ Job = &configureHost{}

// configure host
type configureHost struct {
	*deploymentJob

	ctx    context.Context
	cancel context.CancelFunc
	log    *slog.Logger

	status entity.HostConfigurationStatus
}

func (a *configureHost) Execute() error {
	ctx := a.ctx
	// TODO: separate it into their own module later...
	a.log.Info("configuring host directory")

	if err := ctx.Err(); err != nil {
		a.status = entity.HostConfigurationStatusCancelled
		a.log.Error("job cancelled", "error", err)
		return err
	}

	basePath := fmt.Sprintf("/opt/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)

	a.log.Info("ensuring path", "path", basePath)
	err := ensureDir(basePath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring directory in base path", "path", basePath, "error", err)
		return err
	}

	envPath := fmt.Sprintf(basePath+"/env-release/%v", a.Job.Request.EnvVersion)
	a.log.Info("ensuring path", "path", envPath)
	err = ensureDir(envPath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring env path", "path", envPath, "error", err)
		return err
	}

	etcPath := fmt.Sprintf("/etc/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)
	a.log.Info("ensuring path", "path", etcPath)
	err = ensureDir(etcPath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring etc path", "path", etcPath, "error", err)
		return err
	}

	tmpPath := fmt.Sprintf("/tmp/%s_%s/artifact/%v", a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.BuildVersion)
	a.log.Info("ensuring path", "tmp", tmpPath)
	err = ensureDir(tmpPath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring tmp path", "path", tmpPath, "error", err)
		return err
	}

	systemdPath := "/etc/systemd/system"
	a.log.Info("ensuring path", "path", systemdPath)
	err = ensureDir(systemdPath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err)
		return err
	}

	// write systemd
	a.log.Info("writing unit file")
	if err := ctx.Err(); err != nil {
		a.status = entity.HostConfigurationStatusCancelled
		a.log.Error("job cancelled", "error", err)
		return err
	}

	serviceName := fmt.Sprintf("%v_%v.service", a.Job.Request.Ns, a.Job.Request.Service.Id)

	err = func() error {
		content := BuildUnit(a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.Service.Description, a.Job.Request.Service.ExecutablePath)
		name := serviceName
		tmp := filepath.Join(systemdPath, name+".tmp")
		final := filepath.Join(systemdPath, name)
		if err1 := os.WriteFile(tmp, []byte(content), 0644); err1 != nil {
			a.status = entity.HostConfigurationStatusFailed
			a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err1)
			return err1
		}
		err1 := os.Rename(tmp, final)
		if err1 != nil {
			a.status = entity.HostConfigurationStatusFailed
			a.log.Error("error while ensuring systemd path", "path", systemdPath, "error", err1)
			return err1
		}

		return nil
	}()
	if err != nil {
		return err
	}

	// start more heavier operation
	a.log.Info("downloading .env")
	if err := ctx.Err(); err != nil {
		a.status = entity.HostConfigurationStatusCancelled
		a.log.Error("job cancelled", "error", err)
		return err
	}

	err = func() error {
		envData, err1 := a.dependencies.EnvUsecase.Get(ctx, a.Job.Request.Ns, []string{a.Job.Request.Service.Id}, strconv.FormatUint(a.Job.Request.EnvVersion, 10))
		if err1 != nil || len(envData) == 0 {
			a.status = entity.HostConfigurationStatusFailed
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
			a.status = entity.HostConfigurationStatusFailed
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
		return err
	}

	buildReleasePath := fmt.Sprintf(basePath+"/build-release/%v", a.Job.Request.BuildVersion)
	err = ensureDir(buildReleasePath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring build release path", "path", buildReleasePath, "error", err)
		return err
	}

	// TODO: use per file based check / more robust approach;
	isBuildEmpty, err := isEmptyDir(buildReleasePath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while check existing installation inside", "path", buildReleasePath, "error", err)
		return err
	}

	// TODO: remove this; after finding a way to optimize use installation
	if !isBuildEmpty {
		a.status = entity.HostConfigurationStatusSuccess
		a.log.Info("host is configured")
		return err
	}

	a.log.Info("downloading build artifact")
	if err := ctx.Err(); err != nil {
		a.status = entity.HostConfigurationStatusCancelled
		a.log.Error("job cancelled", "error", err)
		return err
	}

	err = func() error {
		buildId := strconv.FormatUint(a.Job.Request.BuildVersion, 10)

		f, err1 := os.OpenFile(tmpPath+"/release.tar.gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err1 != nil {
			a.status = entity.HostConfigurationStatusFailed
			a.log.Error("error while opening env file", "error", err1)
			return err1
		}
		defer f.Close()

		buildArtifact, meta, err1 := a.dependencies.BuildArtifactUsecase.GetAttachment(
			ctx,
			a.Job.Request.Ns,
			[]string{a.Job.Request.Service.Id, buildId},
			fmt.Sprintf("%v/%v", a.host.OS, a.host.Architecture), // attachment can have one to many, so we're restricting to one
		)
		if err1 != nil {
			a.status = entity.HostConfigurationStatusFailed
			a.log.Error("error while getting build artifact", "error", err1)

			if errors.Is(err1, context.Canceled) {
				a.Status = StatusCancelled
			}

			return err1
		}
		defer buildArtifact.Close()

		// Download
		total, err1 := Copy(ctx, f, buildArtifact)
		if err1 != nil {
			a.status = entity.HostConfigurationStatusFailed
			a.log.Error("error while writing artifact file", "error", err1)
			return err1
		}

		if meta.ContentSize != uint64(total) {
			return fmt.Errorf("download file size not matching! expected %v got %v", meta.ContentSize, total)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	a.log.Info("extracting build artifact")

	tmp := buildReleasePath + ".tmp"

	err = ensureDir(tmp)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while ensuring extracted artifact dir", "error", err)
		return err
	}
	err = os.RemoveAll(tmp)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while removing old artifact", "error", err)
		return err
	}

	err = ExtractTarGzStrip(tmpPath+"/release.tar.gz", tmp)
	if err != nil {
		return fmt.Errorf("error while extracting artifact file: %w", err)
	}

	err = os.RemoveAll(buildReleasePath) // delete previous
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while deleting previous installation", "error", err)
		return err
	}

	err = os.Rename(tmp, buildReleasePath)
	if err != nil {
		a.status = entity.HostConfigurationStatusFailed
		a.log.Error("error while renaming artifact file", "error", err)
		return err
	}

	err = func() error {
		// reload daemon reload
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		conn, err := dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd: %w", err)
		}
		defer conn.Close()

		// This is equivalent to: systemctl daemon-reload
		if err := conn.ReloadContext(ctx); err != nil {
			return fmt.Errorf("failed to reload systemd: %w", err)
		}

		props, err := conn.GetUnitPropertiesContext(ctx, serviceName)
		if err != nil {
			a.status = entity.HostConfigurationStatusFailed
			return err
		}

		loadErr, ok := props["LoadError"].(string)
		if ok && loadErr != "" {
			return fmt.Errorf("systemd library load error: %v", err)
		}

		loadState, ok := props["LoadState"].(string)
		if !ok {
			return errors.New("systemd library error")
		}

		if loadState != "loaded" {
			a.status = entity.HostConfigurationStatusFailed
			return fmt.Errorf("service is not loaded. found '%v' state instead for service '%v'", loadState, serviceName)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	a.status = entity.HostConfigurationStatusSuccess
	a.log.Info("successfully configured host")

	return nil
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
