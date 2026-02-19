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
)

var _ Job = &configureHost{}

// configure host
type configureHost struct {
	*deploymentJob

	ctx    context.Context
	cancel context.CancelFunc
	log    *slog.Logger
}

func (a *configureHost) Execute() error {
	var progress float64
	log := a.log
	ctx := a.ctx
	// TODO: separate it into their own module later...
	log.Info("configuring host directory", "progress", progress)

	if err := ctx.Err(); err != nil {
		return err
	}

	basePath := fmt.Sprintf("/opt/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)

	log.Info("ensuring path", "path", basePath, "progress", progress)
	err := ensureDir(basePath)
	if err != nil {
		return fmt.Errorf("error while ensuring directory in base path %v %w", basePath, err)
	}

	envPath := fmt.Sprintf(basePath+"/env-release/%v", a.Job.Request.EnvVersion)
	log.Info("ensuring path", "path", envPath)
	err = ensureDir(envPath)
	if err != nil {
		return fmt.Errorf("error while ensuring env path %v %w", envPath, err)
	}

	etcPath := fmt.Sprintf("/etc/%v_%v", a.Job.Request.Ns, a.Job.Request.Service.Id)
	log.Info("ensuring path", "path", etcPath)
	err = ensureDir(etcPath)
	if err != nil {
		return fmt.Errorf("error while ensuring etc path %v %w", etcPath, err)
	}

	tmpPath := fmt.Sprintf("/tmp/%s_%s/artifact/%v", a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.BuildVersion)
	log.Info("ensuring path", "tmp", tmpPath)
	err = ensureDir(tmpPath)
	if err != nil {
		return fmt.Errorf("error while ensuring tmp path %v %w", tmpPath, err)
	}

	systemdPath := "/etc/systemd/system"
	log.Info("ensuring path", "path", systemdPath)
	err = ensureDir(systemdPath)
	if err != nil {
		return fmt.Errorf("error while ensuring systemd path %v %w", systemdPath, err)
	}

	// write systemd
	progress = 1 / float64(4)

	log.Info("writing unit file", "progress", progress)
	if err := ctx.Err(); err != nil {
		return err
	}

	serviceName := fmt.Sprintf("%v_%v.service", a.Job.Request.Ns, a.Job.Request.Service.Id)

	err = func() error {
		content := BuildUnit(a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.Service.Description, a.Job.Request.Service.ExecutablePath)
		name := serviceName
		tmp := filepath.Join(systemdPath, name+".tmp")
		final := filepath.Join(systemdPath, name)
		if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
			return fmt.Errorf("error while writing systemd path %v %w", systemdPath, err)
		}

		err := os.Rename(tmp, final)
		if err != nil {
			return fmt.Errorf("error while replacing systemd definition  %v %w", systemdPath, err)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	// start more heavier operation
	log.Info("downloading .env", "progress", progress)
	if err := ctx.Err(); err != nil {
		return err
	}

	err = func() error {
		envData, err := a.dependencies.EnvUsecase.Get(ctx, a.Job.Request.Ns, []string{a.Job.Request.Service.Id}, strconv.FormatUint(a.Job.Request.EnvVersion, 10))
		if err != nil {
			return fmt.Errorf("error while downloading env %w", err)
		}

		if len(envData) == 0 {
			return nil
		}

		env := envData[0]

		tmpEnv := make([]string, 0, len(env.Value))
		for k, v := range env.Value {
			tmpEnv = append(tmpEnv, fmt.Sprintf("%v=%v", strings.ToUpper(k), strconv.Quote(v)))
		}

		sort.Slice(tmpEnv, func(i, j int) bool {
			return strings.Compare(tmpEnv[i], tmpEnv[j]) < 0
		})

		log.Info("writing .env")

		path := envPath + "/overwrite.env"

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("error while opening env file in %v %w", path, err)
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
		return fmt.Errorf("error while ensuring build release path in %v %w", buildReleasePath, err)
	}

	// TODO: use per file based check / more robust approach;
	isBuildEmpty, err := isEmptyDir(buildReleasePath)
	if err != nil {
		return fmt.Errorf("error while ensuring build release path in %v %w", buildReleasePath, err)
	}

	// TODO: remove this; after finding a way to optimize use installation
	if !isBuildEmpty {
		return nil
	}

	progress = 2 / float64(4)

	log.Info("downloading build artifact", "progress", progress)
	if err := ctx.Err(); err != nil {
		return err
	}

	err = func() error {
		buildId := strconv.FormatUint(a.Job.Request.BuildVersion, 10)

		f, err := os.OpenFile(tmpPath+"/release.tar.gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("error while opening env file %w", err)
		}
		defer f.Close()

		buildArtifact, meta, err := a.dependencies.BuildArtifactUsecase.GetAttachment(
			ctx,
			a.Job.Request.Ns,
			[]string{a.Job.Request.Service.Id, buildId},
			fmt.Sprintf("%v/%v", a.host.OS, a.host.Architecture), // attachment can have one to many, so we're restricting to one
		)
		if err != nil {
			return fmt.Errorf("error while getting build artifact: %w", err)
		}
		defer buildArtifact.Close()

		// Download
		total, err := Copy(ctx, f, buildArtifact)
		if err != nil {
			return fmt.Errorf("error while writing artifact file %v %w", buildReleasePath, err)
		}

		if meta.ContentSize != uint64(total) {
			// maybe check hash
			return fmt.Errorf("download file size not matching! expected %v got %v", meta.ContentSize, total)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	progress = 3 / float64(4)
	log.Info("extracting build artifact", "progress", progress)

	tmp := buildReleasePath + ".tmp"

	err = ensureDir(tmp)
	if err != nil {
		return fmt.Errorf("error while ensuring artifact dir %w", err)
	}
	err = os.RemoveAll(tmp)
	if err != nil {
		return fmt.Errorf("error while removing old artifact %w", err)
	}

	err = ExtractTarGzStrip(tmpPath+"/release.tar.gz", tmp)
	if err != nil {
		return fmt.Errorf("error while extracting artifact file: %w", err)
	}

	err = os.RemoveAll(buildReleasePath) // delete previous
	if err != nil {
		return fmt.Errorf("error while deleting previous artifact: %w", err)
	}

	err = os.Rename(tmp, buildReleasePath)
	if err != nil {
		return fmt.Errorf("error while replacing artifact: %w", err)
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
			return fmt.Errorf("error while getting systemd context: %w", err)
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
			return fmt.Errorf("service is not loaded. found '%v' state instead for service '%v'", loadState, serviceName)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	progress = 4 / float64(4)

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
