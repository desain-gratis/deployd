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
	"github.com/spf13/viper"
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

	// Make sure the required release version directory is there in the host

	basePath := fmt.Sprintf("/opt/%s_%s", a.Job.Request.Ns, a.Job.Request.Service.Id)

	log.Info("ensuring service base path")
	err := ensureDir(basePath)
	if err != nil {
		return fmt.Errorf("error while ensuring directory in base path %v %w", basePath, err)
	}

	envPath := fmt.Sprintf(basePath+"/env-release/%d", a.Job.Request.EnvVersion)
	log.Info("ensuring env release path " + envPath)
	err = ensureDir(envPath)
	if err != nil {
		return fmt.Errorf("error while ensuring env path %v %w", envPath, err)
	}

	secretPath := fmt.Sprintf(basePath+"/secret-release/%d", a.Job.Request.SecretVersion)
	log.Info("ensuring secret release path " + secretPath)
	err = ensureDir(secretPath)
	if err != nil {
		return fmt.Errorf("error while ensuring secret path %v %w", secretPath, err)
	}

	raftPath := basePath + "/raft-release" // raft is based on id, and it's a string
	log.Info("ensuring raft release path " + raftPath)
	err = ensureDir(raftPath)
	if err != nil {
		return fmt.Errorf("error while ensuring raft path %v %w", raftPath, err)
	}

	// Make sure that the service has already a current /etc directory

	etcPath := fmt.Sprintf("/etc/%s_%s", a.Job.Request.Ns, a.Job.Request.Service.Id)
	log.Info("ensuring etc (config) path")
	err = ensureDir(etcPath)
	if err != nil {
		return fmt.Errorf("error while ensuring etc path %v %w", etcPath, err)
	}

	// Make sure that the service has already a current /tmp directory for temporary artifact download

	tmpPath := fmt.Sprintf("/tmp/%s_%s/artifact/%d", a.Job.Request.Ns, a.Job.Request.Service.Id, a.Job.Request.BuildVersion)
	log.Info("ensuring path", "tmp", tmpPath)
	err = ensureDir(tmpPath)
	if err != nil {
		return fmt.Errorf("error while ensuring tmp path %v %w", tmpPath, err)
	}

	// Make sure the service's systemd is there

	systemdPath := "/etc/systemd/system"
	log.Info("ensuring path", "path", systemdPath)
	err = ensureDir(systemdPath)
	if err != nil {
		return fmt.Errorf("error while ensuring systemd path %v %w", systemdPath, err)
	}

	// Write the application systemd unit file
	log.Info("writing unit file", "progress", progress)
	if err := ctx.Err(); err != nil {
		return err
	}

	serviceName := fmt.Sprintf("%s_%s.service", a.Job.Request.Ns, a.Job.Request.Service.Id)

	err = func() error {
		content := BuildUnit(
			a.Job.Request.Ns,
			a.Job.Request.Service.Id,
			a.Job.Request.Service.Description,
			a.Job.Request.Service.ExecutablePath,
			envPath,
			secretPath,
			raftPath,
		)
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

	// configure clouudflared if enabled;
	// if not, ignore (either already exist or not)
	log.Info("writing cloudflared unit file if configured", "progress", progress)
	if a.Job.Request.RoutingVersion != nil {
		log.Info(" yes it's configured")
		err = a.optConfigureRouting()
		if err != nil {
			return err
		}
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

		buildData, err := a.dependencies.BuildUsecase.Get(
			ctx,
			a.Job.Request.Service.Ns,
			[]string{a.Job.Request.Service.Repository.ID},
			fmt.Sprintf("%v", a.Job.Request.BuildVersion), // attachment can have one to many, so we're restricting to one
		)
		if err != nil {
			return fmt.Errorf("error while getting build artifact: %w", err)
		}
		build := buildData[0]

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

		// TODO: move all overwrite here instead of systemd service definition
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_HOST", a.host.Host)
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_HOST_REPLICA_ID", a.host.RaftConfig.ReplicaID)
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_SERVICE_BUILD_COMMIT_ID", build.CommitID)
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_SERVICE_BUILD_ID", build.Id)
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_SERVICE_BUILD_DATE", build.PublishedAt)
		fmt.Fprintf(f, "%s=%v\n", "DEPLOYD_SERVICE_BUILD_TAG", build.Tag)

		return nil
	}()
	if err != nil {
		return err
	}

	log.Info("downloading secret as .yaml")
	if err := ctx.Err(); err != nil {
		return err
	}

	err = func() error {
		secretData, err := a.dependencies.SecretUsecase.Get(ctx, a.Job.Request.Ns, []string{a.Job.Request.Service.Id}, strconv.FormatUint(a.Job.Request.SecretVersion, 10))
		if err != nil {
			return fmt.Errorf("error while downloading env %w", err)
		}

		if len(secretData) == 0 {
			return nil
		}

		secret := secretData[0]

		type kv struct {
			key string
			v   string
		}

		tmpSecret := make([]kv, 0, len(secret.Value))
		for k, v := range secret.Value {
			tmpSecret = append(tmpSecret, kv{k, v})
		}

		sort.Slice(tmpSecret, func(i, j int) bool {
			return strings.Compare(tmpSecret[i].key, tmpSecret[j].key) < 0
		})

		log.Info("writing .yaml")

		path := secretPath + "/" + secretYamlFile

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("error while opening env file in %v %w", path, err)
		}
		defer f.Close()

		v := viper.New()
		v.SetConfigType("yaml")

		for _, secret := range tmpSecret {
			// todo validatemaxxing
			v.Set(secret.key, secret.v)
		}

		// append with local connection info for secret;
		// with the same structure as deployd itself

		// todo: we create username & password, and authorize the DB creation name for them
		// for now this is suffice..
		v.Set("storage.clickhouse.replica.address", a.dependencies.HostConfig.LocalClickhouseConfig.Address)
		v.Set("storage.clickhouse.replica.database", fmt.Sprintf("%s__%s", a.Job.Request.Ns, a.Job.Request.Service.Id))
		v.Set("storage.clickhouse.replica.username", a.dependencies.HostConfig.LocalClickhouseConfig.Username)
		v.Set("storage.clickhouse.replica.password", a.dependencies.HostConfig.LocalClickhouseConfig.Password)

		err = v.WriteConfigTo(f)
		if err != nil {
			return fmt.Errorf("failed to write .yaml secret %w", err)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	log.Info("downloading raft config as .yaml")
	if err := ctx.Err(); err != nil {
		return err
	}

	// For consistency, use the host snapshotted in the deploy job request
	currentHost := a.host
	for _, host := range a.Job.Target {
		if host.Host == a.host.Host {
			currentHost = a.host
		}
	}

	err = configureRaft(log, raftPath, currentHost, &a.Job)
	if err != nil {
		return err
	}

	// download archive

	buildReleasePath := fmt.Sprintf(basePath+"/build-release/%d", a.Job.Request.BuildVersion)
	err = ensureDir(buildReleasePath)
	if err != nil {
		return fmt.Errorf("error while ensuring build release path in %v %w", buildReleasePath, err)
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
			fmt.Sprintf("%s/%s", a.host.OS, a.host.Architecture), // attachment can have one to many, so we're restricting to one
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

	log.Info(fmt.Sprintf("remove all at %v", tmp))
	err = os.RemoveAll(tmp)
	if err != nil {
		return fmt.Errorf("error while removing old artifact %w", err)
	}

	err = ExtractTarGzStrip(tmpPath+"/release.tar.gz", tmp)
	if err != nil {
		return fmt.Errorf("error while extracting artifact file: %w", err)
	}

	// TODO: check if there is old version; old version is used

	log.Info(fmt.Sprintf("remove all at %v", buildReleasePath))
	err = os.RemoveAll(buildReleasePath) // delete previous
	if err != nil {
		return fmt.Errorf("error while deleting previous artifact: %w", err)
	}

	log.Info(fmt.Sprintf("replacing the old %v with new %v", tmp, buildReleasePath))
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

		_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, true)
		if err != nil {
			return fmt.Errorf("systemd enable load error: %v", err)
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

// todo refactor to make it more natural, eg. by using context
func configureRaft(log *slog.Logger, raftPath string, currentHost *entity.Host, job *entity.DeploymentJob) error {
	shardsConfig := job.RaftConfig.Shards

	type kv struct {
		key uint64
		v   entity.RaftShardConfig
	}

	// hostConfig := job.Target[0]

	// Use currentHost from config; to make it more consistent
	var hostc *entity.Host
	for _, host := range job.Target {
		if host.Host == currentHost.Host {
			hostc = &host
		}
	}
	if hostc == nil {
		return nil // fmt.Errorf("should not be deployed here.")
	}

	tmpRaftConfig := make([]kv, 0, len(shardsConfig))
	for shardID, v := range shardsConfig {
		tmpRaftConfig = append(tmpRaftConfig, kv{shardID, v})
	}

	sort.Slice(tmpRaftConfig, func(i, j int) bool {
		return int(tmpRaftConfig[i].key)-int(tmpRaftConfig[j].key) < 0
	})

	log.Info("writing .yaml")

	path := raftPath + "/" + dragonboatYamlFile

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error while opening env file in %v %w", path, err)
	}
	defer f.Close()

	v := viper.New()
	v.SetConfigType("yaml")

	raftService := job.RaftConfig.Service[currentHost.Host]

	v.Set("host.replica_id", currentHost.RaftConfig.ReplicaID)
	v.Set("host.raft_address", raftService.RaftAddress)
	v.Set("host.wal_dir", fmt.Sprintf("%s/%s_%s", currentHost.RaftConfig.BaseWALDir, job.Ns, job.Request.Service.Id))
	v.Set("host.nodehost_dir", fmt.Sprintf("%s/%s_%s", currentHost.RaftConfig.BaseNodeHostDir, job.Ns, job.Request.Service.Id))
	v.Set("host.deployment_id", raftService.DeploymentID)

	peers := make(map[uint64]string)
	for _, peer := range job.RaftConfig.Service {
		peers[peer.ReplicaID] = peer.RaftAddress
	}
	v.Set("host.peer", peers)

	for shardID, shardConfig := range job.RaftConfig.Shards {
		var isBootstrap bool
		if shardConfig.BootstrapHost == currentHost.Host {
			isBootstrap = true
		}

		// todo: find more elegant way..
		// _ = shardID
		// _ = isBootstrap
		v.Set(fmt.Sprintf("replica.%v.shard_id", shardID), shardID)
		v.Set(fmt.Sprintf("replica.%v.bootstrap", shardID), isBootstrap)
		v.Set(fmt.Sprintf("replica.%v.id", shardID), shardConfig.ID)
		v.Set(fmt.Sprintf("replica.%v.shard_id", shardID), shardID)
		v.Set(fmt.Sprintf("replica.%v.alias", shardID), shardConfig.Description)
		v.Set(fmt.Sprintf("replica.%v.description", shardID), shardConfig.Description)
		v.Set(fmt.Sprintf("replica.%v.type", shardID), shardConfig.Type)
	}

	err = v.WriteConfigTo(f)
	if err != nil {
		return fmt.Errorf("failed to write .yaml raft %w", err)
	}

	return nil
}
