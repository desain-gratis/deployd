package artifactd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/desain-gratis/common/lib/raft"
	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
	"github.com/rs/zerolog/log"
)

const appName = "artifactd"

var _ raft.Application = &app{}

type app struct {
	state *state
}

type Namespace struct {
	Namespace string
	Name      string
}

type state struct {
	Index map[Namespace]*uint64
}

func NewRaft() *app {
	return &app{}
}

// Init
func (a *app) Init(ctx context.Context) error {

	conn := raft_runner.GetClickhouseConnection(ctx)

	err := conn.Exec(ctx, ddlCommit)
	if err != nil {
		log.Panic().Msgf("panic creating table: %v", err)
	}

	// load metadata

	// get metadata
	meta, err := raft_runner.GetMetadata(ctx, appName)
	if err != nil {
		return err
	}

	a.state = &state{}

	if len(meta) > 0 {
		err = json.Unmarshal(meta, a.state)
		if err != nil {
			return err
		}
	}

	if a.state.Index == nil {
		a.state.Index = make(map[Namespace]*uint64)
	}

	return nil
}

// PrepareApply is to prepare for update scoped resource
func (a *app) PrepareUpdate(ctx context.Context) (context.Context, context.CancelFunc, error) {
	ctx = clickhouse.Context(ctx, clickhouse.WithAsync(true))
	ctx, cancel := context.WithCancel(ctx)

	return ctx, cancel, nil
}

// OnUpdate but before apply
func (a *app) OnUpdate(ctx context.Context, e raft.Entry) raft.OnAfterApply {
	cmd, err := parseJsonAs[Command](e.Cmd)
	if err != nil {
		return responsef("invalid command json: %v", cmd)
	}

	switch cmd.Command {
	case "register-artifact":
		data, err := parseJsonAs[Artifact](cmd.Data)
		if err != nil {
			return responsef("invalid register artifact input: %v", cmd)
		}
		return a.registerArtifact(ctx, data)
	}

	return responsef("unsupported commands: %v", cmd.Command)
}

// Apply to place the code to commit to disk or "Sync"
func (a *app) Apply(ctx context.Context) error {
	payload, _ := json.Marshal(a.state)
	err := raft_runner.SetMetadata(ctx, appName, payload)
	if err != nil {
		return err
	}

	return nil
}

// Lookup
func (a *app) Lookup(ctx context.Context, key interface{}) (interface{}, error) {
	conn := raft_runner.GetClickhouseConnection(ctx)

	k, ok := key.(QueryArtifact)
	if !ok {
		return "bad", nil
	}

	rows, err := conn.Query(ctx, dqlGetCommit, k.Namespace, k.Name, k.From)
	if err != nil {
		return nil, err
	}

	out := make(chan *Artifact)

	go func() {
		defer close(out)
		defer rows.Close()

		for rows.Next() {
			var artifact Artifact
			// namespace, name, commit_id, branch, tag, actor, data, created_at
			err := rows.ScanStruct(&artifact)
			if err != nil {
				log.Err(err).Msgf("error scan")
				continue
			}

			out <- &artifact
		}
	}()

	return (<-chan *Artifact)(out), nil
}

func (a *app) registerArtifact(ctx context.Context, data *Artifact) raft.OnAfterApply {
	conn := raft_runner.GetClickhouseConnection(ctx)

	nsKey := Namespace{Namespace: data.Namespace, Name: data.Name}
	_, ok := a.state.Index[nsKey]
	if !ok {
		var nidx uint64
		a.state.Index[nsKey] = &nidx
	}

	idx := *a.state.Index[nsKey]

	// (namespace, name, commit_id, branch, tag, actor, data, published_at)
	err := conn.Exec(ctx, dmlRegisterCommit,
		data.Namespace, data.Name, idx, data.CommitID, data.Branch, data.Tag, data.Actor, string(data.Data), time.Now(), data.Source, data.OsArch,
	)
	if err != nil {
		return responsef("error: %v", err)
	}

	*a.state.Index[nsKey]++

	return responsef("success")
}

func parseJsonAs[T any](data []byte) (*T, error) {
	var t T
	err := json.Unmarshal(data, &t)
	return &t, err
}

func responsef(format string, a ...any) raft.OnAfterApply {
	return func() (raft.Result, error) {
		return raft.Result{Data: []byte(fmt.Sprintf(format, a...))}, nil
	}
}
