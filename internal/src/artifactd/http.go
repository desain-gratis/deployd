package artifactd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/client"

	raft_runner "github.com/desain-gratis/common/lib/raft/runner"
)

type httpHandler struct {
	shardID   uint64
	replicaID uint64
	dhost     *dragonboat.NodeHost
	sess      *client.Session
}

func (h *httpHandler) StreamAll(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

}

func NewClient(ctx context.Context) *httpHandler {
	rctx := raft_runner.GetRaftContext(ctx)
	sess := rctx.DHost.GetNoOPSession(rctx.ShardID)

	return &httpHandler{
		shardID:   rctx.ShardID,
		replicaID: rctx.ReplicaID,
		dhost:     rctx.DHost,
		sess:      sess,
	}
}

func Query[T any](ctx context.Context, contextKey string) (<-chan T, error) {
	rctx := raft_runner.GetRaftContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, 4000*time.Millisecond)
	defer cancel()

	result, err := rctx.DHost.SyncRead(ctx, rctx.ShardID, ctx.Value(contextKey))

	if err != nil {
		return nil, err
	}

	stream, ok := result.(<-chan T)
	if !ok {
		return nil, fmt.Errorf("not expected data type: %t", result)
	}

	return stream, nil
}

func (h *httpHandler) RegisterArtifact(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	reader := http.MaxBytesReader(w, r.Body, 1000000)
	payload, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}

	// validate etc.
	var req Artifact
	err = json.Unmarshal(payload, &req)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}

	cmd := Command{
		Command: "register-artifact",
		Data:    payload,
	}

	raftReq, err := json.Marshal(cmd)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 4000*time.Millisecond)
	defer cancel()

	result, err := h.dhost.SyncPropose(ctx, h.sess, raftReq)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err) // todo json
		return
	}

	fmt.Fprintf(w, string(result.Data))
}

func (h *httpHandler) DiscoverArtifact(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 4000*time.Millisecond)
	defer cancel()

	result, err := h.dhost.SyncRead(ctx, h.shardID, QueryArtifact{
		Namespace: r.URL.Query().Get("namespace"),
		Name:      r.URL.Query().Get("name"),
		From:      time.Now().AddDate(0, 0, -7),
	})

	if err != nil {
		fmt.Fprintf(w, "error: %v", err) // todo json
		return
	}

	stream, ok := result.(<-chan *Artifact)
	if !ok {
		fmt.Fprintf(w, "error: not expected data %t", result) // todo json
		return
	}

	for data := range stream {
		d, _ := json.Marshal(data)
		fmt.Fprintf(w, "%+v\n", string(d))
	}
}

func GetFilter(req *http.Request, p httprouter.Param) QueryArtifact {
	return QueryArtifact{}
}

func (h *httpHandler) Discover(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := context.WithValue(r.Context(), "filter", QueryArtifact{
			Namespace: r.URL.Query().Get("namespace"),
			Name:      r.URL.Query().Get("name"),
			From:      time.Now().AddDate(0, 0, -7),
		})

		handler(w, r.WithContext(ctx), p)
	}
}
