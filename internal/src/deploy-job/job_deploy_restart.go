package deployjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

const (
	overwriteEnvFile   = "overwrite.env"
	dragonboatYamlFile = "dragonboat.yaml"
	secretYamlFile     = "secret.yaml"
)

var _ Job = &restartHostService{}

// restart host service
type restartHostService struct {
	*deploymentJob

	ctx    context.Context
	cancel context.CancelFunc
	log    *slog.Logger
}

func (c *restartHostService) Execute() error {
	// drain tunnel
	// stop
	// modify link
	// start
	// wait ready
	// start tunnel

	config := DeployConfig{
		ServiceID: c.Job.Request.Service.Id,
		Namespace: c.Job.Request.Service.Ns,

		BuildID:       strconv.FormatUint(c.Job.Request.BuildVersion, 10),
		EnvVersion:    strconv.FormatUint(c.Job.Request.EnvVersion, 10),
		SecretVersion: strconv.FormatUint(c.Job.Request.SecretVersion, 10),
		BaseDir:       "/opt",
		BinPath:       c.Job.Request.Service.ExecutablePath,
		Timeout:       8 * time.Hour,
	}

	return Deploy(c.log, c.ctx, config)

	// return errors.New("not implemented yet")
}

type DeployConfig struct {
	Namespace     string
	ServiceID     string
	BuildID       string // e.g. "20260215-abc123"
	EnvVersion    string
	SecretVersion string
	BaseDir       string        // default: /opt
	BinPath       string        // e.g. "bin/myapp"
	Timeout       time.Duration // optional
}

func Deploy(log *slog.Logger, ctx context.Context, cfg DeployConfig) error {
	if cfg.Namespace == "" || cfg.ServiceID == "" || cfg.BuildID == "" {
		return errors.New("missing service namespace, name or build id")
	}

	if cfg.BaseDir == "" {
		cfg.BaseDir = "/opt"
	}

	serviceName := fmt.Sprintf("%s_%s", cfg.Namespace, cfg.ServiceID)

	baseDir := filepath.Join(cfg.BaseDir, serviceName)
	releaseDir := filepath.Join(baseDir, "build-release", cfg.BuildID)
	envReleaseDir := filepath.Join(baseDir, "env-release", cfg.EnvVersion)
	secretReleaseDir := filepath.Join(baseDir, "secret-release", cfg.SecretVersion)
	raftReleaseDir := filepath.Join(baseDir, "raft-release")

	currentLink := filepath.Join(baseDir, "current")

	etcServiceDir := filepath.Join("/etc", serviceName)
	etcEnvLink := filepath.Join(etcServiceDir, "env")
	etcSecretLink := filepath.Join(etcServiceDir, "secret")
	etcRaftLink := filepath.Join(etcServiceDir, "raft")

	unitName := serviceName + ".service"
	cloudflaredUnitName := fmt.Sprintf("cloudflared_%s_%s.service", cfg.Namespace, cfg.ServiceID)

	// 1️⃣ Validate release exists
	releaseInfo, err := os.Stat(releaseDir)
	if err != nil || !releaseInfo.IsDir() {
		return fmt.Errorf("release directory not found: %s", releaseDir)
	}

	// 2️⃣ Validate env-release exists
	envInfo, err := os.Stat(envReleaseDir)
	if err != nil || !envInfo.IsDir() {
		return fmt.Errorf("env-release directory not found: %s", envReleaseDir)
	}

	// Validate overwrite.env exists
	overwriteEnvPath := filepath.Join(envReleaseDir, overwriteEnvFile)
	if _, err := os.Stat(overwriteEnvPath); err != nil {
		return fmt.Errorf("overwrite.env not found in env-release: %s", overwriteEnvPath)
	}

	// Validate secret-release exists (if provided)
	if cfg.SecretVersion != "" {
		secretInfo, err := os.Stat(secretReleaseDir)
		if err != nil || !secretInfo.IsDir() {
			return fmt.Errorf("secret-release directory not found: %s", secretReleaseDir)
		}
	}

	// Validate raft-release exists (always provided)
	raftInfo, err := os.Stat(raftReleaseDir)
	if err != nil || !raftInfo.IsDir() {
		return fmt.Errorf("raft-release directory not found: %s", raftReleaseDir)
	}

	// 3️⃣ Validate binary exists
	binaryPath := filepath.Join(releaseDir, cfg.BinPath)

	binInfo, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Ensure owner execute bit (chmod u+x)
	mode := binInfo.Mode()
	if mode&0100 == 0 {
		if err := os.Chmod(binaryPath, mode|0100); err != nil {
			return fmt.Errorf("failed to chmod u+x on %s: %w", binaryPath, err)
		}
	}

	// 4️⃣ Ensure /etc/<service> exists
	if err := os.MkdirAll(etcServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create etc service dir: %w", err)
	}

	// 5️⃣ Connect to systemd
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// TODO: currently only for cloudflare

	// if _, err := os.Stat(binaryPath)
	// if ServiceFileExists()
	found, err := ServiceFileExists(ctx, conn, cloudflaredUnitName)
	if err != nil {
		return fmt.Errorf("failed to check cloudflared systemd config exist or not %w", err)
	}
	if found {
		log.Info("draining cloudflare connection")
		// 6️⃣ Stop cf
		if err := stopService(ctx, conn, cloudflaredUnitName); err != nil {
			log.Warn(fmt.Sprintf("warn cloudflared %v", err)) // TODO use slog, differentiate error vs notfound
		}
		// after stop & draining traffic.. we go

		defer func() {
			log.Info("starting cloudflare routing")
			if err := startService(ctx, conn, cloudflaredUnitName); err != nil {
				log.Warn(fmt.Sprintf("warn startcloudflared %v", err))
			}
		}()
	}

	// todo all refactorr2

	// 6️⃣ Stop service
	if err := stopService(ctx, conn, unitName); err != nil {
		return err
	}

	log.Info("service stopped")

	// TODO: refactor this for revert logic cancel, etc.

	log.Info("updating links")

	// 7️⃣ Backup previous symlinks for rollback
	prevBuildTarget, _ := os.Readlink(currentLink)
	prevEnvTarget, _ := os.Readlink(etcEnvLink)
	prevSecretTarget, _ := os.Readlink(etcSecretLink)

	// 8️⃣ Switch build symlink
	if err := switchSymlinkAtomic(currentLink, releaseDir); err != nil {
		return err
	}

	// 9️⃣ Switch env symlink
	if err := switchSymlinkAtomic(etcEnvLink, envReleaseDir); err != nil {
		// rollback build if env switch fails
		if prevBuildTarget != "" {
			_ = switchSymlinkAtomic(currentLink, prevBuildTarget)
		}
		return fmt.Errorf("failed to switch env symlink: %w", err)
	}

	// Switch secret symlink
	if cfg.SecretVersion != "" {
		if err := switchSymlinkAtomic(etcSecretLink, secretReleaseDir); err != nil {
			rollbackAll(currentLink, prevBuildTarget,
				etcEnvLink, prevEnvTarget,
				etcSecretLink, prevSecretTarget,
				conn, ctx, unitName)
			return fmt.Errorf("failed to switch secret symlink: %w", err)
		}
	}

	// Switch raft symlink
	if err := switchSymlinkAtomic(etcRaftLink, raftReleaseDir); err != nil {
		rollbackAll(currentLink, prevBuildTarget,
			etcEnvLink, prevEnvTarget,
			etcSecretLink, prevSecretTarget,
			conn, ctx, unitName)
		return fmt.Errorf("failed to switch raft symlink: %w", err)
	}

	log.Info("links updated")

	// 🔟 Start service
	log.Info(fmt.Sprintf("starting service %v ...", unitName))
	if err := startService(ctx, conn, unitName); err != nil {
		rollbackAll(currentLink, prevBuildTarget,
			etcEnvLink, prevEnvTarget,
			etcSecretLink, prevSecretTarget,
			conn, ctx, unitName)
		return fmt.Errorf("start failed, rolled back: %w", err)
	}

	// 1️⃣1️⃣ Verify active state
	log.Info(fmt.Sprintf("verifying that the service is active %v ...", unitName))
	active, err := isActive(ctx, conn, unitName)
	log.Info(fmt.Sprintf("verifying that the service is active ... %v %v %v", unitName, active, err))
	if err != nil || !active {
		rollbackAll(currentLink, prevBuildTarget,
			etcEnvLink, prevEnvTarget,
			etcSecretLink, prevSecretTarget,
			conn, ctx, unitName)
		return fmt.Errorf("service failed health check after start: %v", err)
	}

	// TODO: add healthcheck
	log.Info("healthcheck not implemented yet")

	return nil
}

func stopService(ctx context.Context, conn *dbus.Conn, name string) error {
	ch := make(chan string, 1)

	_, err := conn.StopUnitContext(ctx, name, "replace", ch)
	if err != nil {
		return err
	}

	select {
	case <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func startService(ctx context.Context, conn *dbus.Conn, name string) error {
	ch := make(chan string, 1)

	_, err := conn.StartUnitContext(ctx, name, "replace", ch)
	if err != nil {
		return err
	}

	select {
	case <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func isActive(ctx context.Context, conn *dbus.Conn, name string) (bool, error) {
	props, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return false, err
	}

	state, ok := props["ActiveState"].(string)
	if !ok {
		return false, errors.New("ActiveState missing")
	}

	return state == "active", nil
}

func switchSymlinkAtomic(linkPath, newTarget string) error {
	tmpLink := linkPath + ".tmp"

	_ = os.Remove(tmpLink)

	if err := os.Symlink(newTarget, tmpLink); err != nil {
		return err
	}

	return os.Rename(tmpLink, linkPath)
}

func rollbackAll(
	buildLink, prevBuild string,
	envLink, prevEnv string,
	secretLink, prevSecret string,
	conn *dbus.Conn,
	ctx context.Context,
	unit string,
) {
	if prevBuild != "" {
		_ = switchSymlinkAtomic(buildLink, prevBuild)
	}
	if prevEnv != "" {
		_ = switchSymlinkAtomic(envLink, prevEnv)
	}
	if prevSecret != "" {
		_ = switchSymlinkAtomic(secretLink, prevSecret)
	}

	_ = startService(ctx, conn, unit)
}

func ServiceFileExists(ctx context.Context, conn *dbus.Conn, name string) (bool, error) {
	files, err := conn.ListUnitFilesByPatternsContext(ctx, []string{}, []string{name})
	if err != nil {
		return false, err
	}

	return len(files) > 0, nil
}
