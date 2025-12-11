package artifactd

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
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
