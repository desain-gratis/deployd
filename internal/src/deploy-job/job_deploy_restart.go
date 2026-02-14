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
	"github.com/desain-gratis/deployd/src/entity"
)

var _ Job = &restartHostService{}

// restart host service
type restartHostService struct {
	*deploymentJob

	ctx    context.Context
	cancel context.CancelFunc
	log    *slog.Logger

	status entity.HostDeploymentStatus
}

func (c *restartHostService) Execute() error {
	// drain tunnel
	// stop
	// modify link
	// start
	// wait ready
	// start tunnel

	config := DeployConfig{
		ServiceName: fmt.Sprintf("%v_%v", c.Job.Ns, c.Job.Request.Service.Id),
		BuildID:     strconv.FormatUint(c.Job.Request.BuildVersion, 10),
		EnvVersion:  strconv.FormatUint(c.Job.Request.EnvVersion, 10),
		BaseDir:     "/opt",
		BinPath:     c.Job.Request.Service.ExecutablePath,
		Timeout:     30 * time.Hour,
	}

	return Deploy(c.ctx, config)

	// return errors.New("not implemented yet")
}

type DeployConfig struct {
	ServiceName string // e.g. "deployd_user-profile"
	BuildID     string // e.g. "20260215-abc123"
	EnvVersion  string
	BaseDir     string        // default: /opt
	BinPath     string        // e.g. "bin/myapp"
	Timeout     time.Duration // optional
}

func Deploy(ctx context.Context, cfg DeployConfig) error {
	if cfg.ServiceName == "" || cfg.BuildID == "" {
		return errors.New("missing service name or build id")
	}

	if cfg.BaseDir == "" {
		cfg.BaseDir = "/opt"
	}

	baseDir := filepath.Join(cfg.BaseDir, cfg.ServiceName)
	releaseDir := filepath.Join(baseDir, "build-release", cfg.BuildID)
	envReleaseDir := filepath.Join(baseDir, "env-release", cfg.EnvVersion)

	currentLink := filepath.Join(baseDir, "current")

	etcServiceDir := filepath.Join("/etc", cfg.ServiceName)
	etcEnvLink := filepath.Join(etcServiceDir, "env")

	unitName := cfg.ServiceName + ".service"

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
	overwriteEnvPath := filepath.Join(envReleaseDir, "overwrite.env")
	if _, err := os.Stat(overwriteEnvPath); err != nil {
		return fmt.Errorf("overwrite.env not found in env-release: %s", overwriteEnvPath)
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

	// 6️⃣ Stop service
	if err := stopService(ctx, conn, unitName); err != nil {
		return err
	}

	// 7️⃣ Backup previous symlinks for rollback
	prevBuildTarget, _ := os.Readlink(currentLink)
	prevEnvTarget, _ := os.Readlink(etcEnvLink)

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

	// 🔟 Start service
	if err := startService(ctx, conn, unitName); err != nil {
		rollback(currentLink, prevBuildTarget, etcEnvLink, prevEnvTarget, conn, ctx, unitName)
		return fmt.Errorf("start failed, rolled back: %w", err)
	}

	// 1️⃣1️⃣ Verify active state
	active, err := isActive(ctx, conn, unitName)
	if err != nil || !active {
		rollback(currentLink, prevBuildTarget, etcEnvLink, prevEnvTarget, conn, ctx, unitName)
		return fmt.Errorf("service failed health check after start: %v", err)
	}

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

func rollback(
	buildLink, prevBuild,
	envLink, prevEnv string,
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
	_ = startService(ctx, conn, unit)
}
