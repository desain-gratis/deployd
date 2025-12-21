package artifactd

import (
	"net/http"

	"github.com/desain-gratis/common/lib/notifier"
	"github.com/julienschmidt/httprouter"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/client"
)

type httpHandler struct {
	*handler
}

func Http(handler *handler) *httpHandler {
	return &httpHandler{
		handler: handler,
	}
}

func (h *httpHandler) StreamAll(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

}

func HttpRaftHandler(outbox notifier.Topic, shardID, replicaID uint64, Dhost *dragonboat.NodeHost, Sess *client.Session) *raftHandler {
	return nil
}
