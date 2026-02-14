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
		BaseDir:     "/opt",
		BinPath:     c.Job.Request.Service.ExecutablePath,
		Timeout:     30 * time.Hour,
	}

	return Deploy(c.ctx, config)

	// return errors.New("not implemented yet")
}

type DeployConfig struct {
	ServiceName string // e.g. "myapp"
	BuildID     string // e.g. "20260215-abc123"
	BaseDir     string // default: /opt
	BinPath     string // e.g. "bin/myapp" (relative to release root)
	Timeout     time.Duration
}

func Deploy(ctx context.Context, cfg DeployConfig) error {
	if cfg.ServiceName == "" || cfg.BuildID == "" {
		return errors.New("missing service name or build id")
	}

	baseDir := filepath.Join(cfg.BaseDir, cfg.ServiceName)
	releaseDir := filepath.Join(baseDir, "build-release", cfg.BuildID)
	currentLink := filepath.Join(baseDir, "current")
	unitName := cfg.ServiceName + ".service"

	// 1️⃣ Validate release exists
	info, err := os.Stat(releaseDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("release directory not found: %s", releaseDir)
	}

	// 2️⃣ Validate binary exists
	binaryPath := filepath.Join(releaseDir, cfg.BinPath)
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Ensure owner execute bit (chmod u+x)
	mode := info.Mode()
	if mode&0100 == 0 { // owner execute not set
		newMode := mode | 0100
		if err := os.Chmod(binaryPath, newMode); err != nil {
			return fmt.Errorf("failed to chmod u+x on %s: %w", binaryPath, err)
		}
	}

	// 3️⃣ Connect to systemd
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 4️⃣ Stop service
	if err := stopService(ctx, conn, unitName); err != nil {
		return err
	}

	// 5️⃣ Backup previous symlink for rollback
	prevTarget, _ := os.Readlink(currentLink)

	// 6️⃣ Atomic symlink switch
	if err := switchSymlinkAtomic(currentLink, releaseDir); err != nil {
		return err
	}

	// 7️⃣ Start service
	if err := startService(ctx, conn, unitName); err != nil {
		rollback(currentLink, prevTarget, conn, ctx, unitName)
		return fmt.Errorf("start failed, rolled back: %w", err)
	}

	// 8️⃣ Verify active
	active, err := isActive(ctx, conn, unitName)
	if err != nil || !active {
		rollback(currentLink, prevTarget, conn, ctx, unitName)
		return fmt.Errorf("service failed health check after start")
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

func rollback(linkPath, prevTarget string, conn *dbus.Conn, ctx context.Context, unit string) {
	if prevTarget == "" {
		return
	}

	_ = switchSymlinkAtomic(linkPath, prevTarget)
	_ = startService(ctx, conn, unit)
}
