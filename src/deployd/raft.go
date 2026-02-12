package deployd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	mycontentapiclient "github.com/desain-gratis/common/delivery/mycontent-api-client"
	raft_replica "github.com/desain-gratis/common/lib/raft/replica"

	"github.com/desain-gratis/deployd/src/entity"
)

const (
	envRaftConfigPath = "DEPLOYD_RAFT"
	envRaftConfigURL  = "DEPLOYD_RAFT_CONFIG"
)

// TODO: add param that are user facing (eg. deployment ID, preffered port)
func InitializeRaftOld(replica map[string]raft_replica.ReplicaConfig) error {
	raftConfig := os.Getenv(envRaftConfigURL)

	if raftConfig == "" {
		return raft_replica.Init()
	}

	// look up to desain gra

	raftConfig = os.Getenv(envRaftConfigPath)
	if raftConfig == "" {
		return raft_replica.Init()
	}

	return raft_replica.InitWithConfigFile(raftConfig, replica)
}

type HostRaftConfig struct {
	ReplicaID uint64   `json:"replica_id"`
	Hosts     []string `json:"hosts"`
}

func InitializeRaft(replica map[uint64]entity.ReplicaConfig) error {
	// TODO: maybe all will hit the DEPLOYD root endpoint
	namespace := os.Getenv("DEPLOYD_NAMESPACE")
	service := os.Getenv("DEPLOYD_SERVICE")
	host := os.Getenv("DEPLOYD_HOST")

	deploydAPI := os.Getenv("DEPLOYD_API")
	auth := os.Getenv("DEPLOYD_API_AUTH")

	if deploydAPI != "" {
		return useDeployd(deploydAPI, auth, namespace, host, service)
	}

	return useConfig()
}

// Stand-alone raft application, use /etc/dragonboat.yaml
func useConfig() error {
	return raft_replica.Init()
}

func useDeployd(deploydAPI, auth, namespace, host, service string) error {

	hostConfigClient := mycontentapiclient.New[*entity.Host](http.DefaultClient, deploydAPI+"/deployd/host", nil, auth)
	raftHostConfigClient := mycontentapiclient.New[*entity.RaftHost](http.DefaultClient, deploydAPI+"/deployd/raft/host", []string{"service"}, auth)
	raftReplicaConfigClient := mycontentapiclient.New[*entity.RaftReplica](http.DefaultClient, deploydAPI+"/deployd/raft/replica", []string{"service"}, auth)

	ctx := context.Background()

	// Get preferred raft host configuration

	var hostConfig entity.Host

	hostConfigs, err := hostConfigClient.Get(ctx, namespace, nil, host)
	if err != nil {
		return fmt.Errorf("deployd API (host config): %v", err)
	}
	if len(hostConfigs) == 0 {
		return fmt.Errorf("deployd API  (host config): empty configuration")
	}
	hostConfig = *hostConfigs[0]

	_ = generateDefaultRaftConfig(hostConfig)

	// raftReplicaConfig := generateDefaultReplicaConfig()
	//
	raftHostConfigs, err := raftHostConfigClient.Get(ctx, namespace, nil, service)
	if err != nil {
		return fmt.Errorf("deployd API (raft host config): %v", err)
	}
	if len(raftHostConfigs) == 0 {
		return fmt.Errorf("deployd API (raft host config): empty configuration")
	}

	raftReplicaConfigs, err := raftReplicaConfigClient.Get(ctx, namespace, nil, service)
	if err != nil {
		return fmt.Errorf("deployd API (raft replica config): %v", err)
	}
	if len(raftReplicaConfigs) == 0 {
		return fmt.Errorf("deployd API (raft replica config): empty configuration")
	}

	return nil

	// hostConfig := hostConfigs[0]
	// raftHostConfig := raftHostConfigs[0]
	// raftReplicaConfig := raftReplicaConfigs[0]

	// defaultConfig := config.NodeHostConfig{
	// 	RaftAddress:    raftHostConfig.RaftAdress,
	// 	WALDir:         raftHostConfig.WALDir,
	// 	NodeHostDir:    os.Getenv("DEPLOYD_RAFT_NODE_HOST_DIR"),
	// 	RTTMillisecond: 100,
	// 	DeploymentID:   0,
	// }

	// var needSync bool

	// if len(result) == 1 {
	// 	cfg := result[0]
	// 	defaultConfig = config.NodeHostConfig{
	// 		RaftAddress:    cfg.RaftAdress,
	// 		WALDir:         cfg.WALDir,
	// 		NodeHostDir:    cfg.NodeHostDir,
	// 		RTTMillisecond: cfg.RTTMillisecond,
	// 		DeploymentID:   cfg.DeploymentID,
	// 	}
	// 	needSync = true
	// }

	// // todo: consider
	// err := raft_replica.Run(replicaID, defaultConfig)
	// if err != nil {
	// 	return err
	// }

	// if needSync {
	// 	// "bake"/snapshot the raft host configuration
	// 	// for first time init in this host, for this service.
	// 	_, err = raftHostConfigClient.Post(ctx, auth, &entity.RaftHost{
	// 		Ns:        namespace,
	// 		Id:        host,
	// 		ServiceID: service,

	// 		ReplicaID:      replicaID,
	// 		RaftAdress:     raftAddress,
	// 		WALDir:         WALDir,
	// 		NodeHostDir:    nodeHostDir,
	// 		RTTMillisecond: 100,
	// 		DeploymentID:   defaultConfig.DeploymentID,

	// 		PublishedAt: time.Now(),
	// 		URLx:        "",
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// // TODO: fill configu
	// configureReplica(ctx, "", namespace, service, replica, "")
}

func generateDefaultRaftConfig(config entity.Host) entity.RaftHost {
	return entity.RaftHost{}
}

func configureReplica(
	ctx context.Context,
	auth string,
	namespace string,
	serviceID string,
	replica map[uint64]entity.ReplicaConfig,
	raftHostConfigUrl string,
) error {
	raftReplicaClient := mycontentapiclient.New[*entity.RaftReplica](http.DefaultClient, raftHostConfigUrl, []string{"id"}, auth)

	remoteReplicaConfig, err := raftReplicaClient.Get(ctx, namespace, nil, serviceID)
	if err != nil {
		return err
	}

	var needSync bool
	if len(remoteReplicaConfig) == 1 {
		replica = remoteReplicaConfig[0].ReplicaConfig
	} else {
		needSync = true
	}

	raft_replica.ConfigureReplica(convertToDGReplica(replica))

	if needSync {
		// bake the cnofig:
		_, err := raftReplicaClient.Post(ctx, &entity.RaftReplica{
			// TODO:
		}, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func convertToDGReplica(replica map[uint64]entity.ReplicaConfig) map[uint64]raft_replica.ReplicaConfig {
	result := make(map[uint64]raft_replica.ReplicaConfig)

	for shardID, r := range replica {
		result[shardID] = raft_replica.ReplicaConfig{
			ShardID:   shardID,
			ReplicaID: r.ReplicaID,
			Bootstrap: false, // TODOOOO TODOTDODOT DOTDO:
			// ReplicaID: ,
		}
	}

	return result
}
