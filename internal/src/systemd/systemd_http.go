package systemd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	notifier_impl "github.com/desain-gratis/common/lib/notifier/impl"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
)

type httpHandler struct {
	*handler
}

func Http(handler *handler) *httpHandler {
	return &httpHandler{
		handler: handler,
	}
}

func (h *httpHandler) StreamUnit(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := r.Context()

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"http://localhost:*", "http://localhost",
			"https://chat.desain.gratis", "http://dxb-keenan.tailnet-ee99.ts.net",
			"https://dxb-keenan.tailnet-ee99.ts.net",
			"https://mb.desain.gratis",
		},
	})
	if err != nil {
		log.Error().Msgf("error accept %v", err)
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "super duper X")

	wsWg := r.Context().Value("ws-wg").(*sync.WaitGroup)
	wsWg.Add(1)
	defer wsWg.Done()

	lctx, lcancel := context.WithCancel(ctx)
	pctx, pcancel := context.WithCancel(context.Background())

	// Reader goroutine, detect client connection close as well.
	go func() {
		// close both listener & publisher ctx if client is the one closing
		defer lcancel()
		defer pcancel()

		for {
			t, payload, err := c.Read(pctx)
			if websocket.CloseStatus(err) > 0 {
				return
			}
			if err != nil {
				// log.Warn().Msgf("unknown error. closing connection: %v")
				return
			}

			if t == websocket.MessageBinary {
				log.Info().Msgf("cannot read la")
				continue
			}

			log.Info().Msgf("uhui: %v", string(payload))
			err = h.parseMessage(payload)
			if err != nil {
				log.Err(err).Msgf("error parsing command")
				continue
			}

			// since we don't read anything
			// err = b.parseMessage(pctx, c, sessID, payload)
			// if err != nil {
			// 	if errors.Is(err, context.Canceled) {
			// 		return
			// 	}

			// 	log.Err(err).Msgf("unknown error. closing connection")
			// 	return
			// }
		}
	}()

	// simple protection (to state machine) against quick open-close connection
	time.Sleep(100 * time.Millisecond)
	if pctx.Err() != nil || lctx.Err() != nil {
		return
	}

	// subscribe until server closed / client closed
	subscribeCtx, cancelSubscribe := context.WithCancelCause(context.Background())

	// merge context
	go func() {
		select {
		case <-lctx.Done():
			cancelSubscribe(errors.New("server closed"))
		case <-pctx.Done():
			cancelSubscribe(errors.New("client closed"))
		}
	}()

	subscription, err := h.topic.Subscribe(subscribeCtx, notifier_impl.NewStandardSubscriber(nil))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Err(err).Msgf("error get listener %v", err)
		return
	}

	// important to start the subscription..
	subscription.Start()

	// send initial list
	func() {
		h.mu.RLock()
		defer h.mu.RUnlock()

		log.Info().Msgf("GACOR: %v", len(h.status))
		for _, unit := range h.status {
			// barbaric doing IO while aquiring read lock :)
			err = h.sendToClient(pctx, c, Row[DBusUnitStatus]{
				Name: "unit",
				Key:  unit.Name,
				Data: *unit,
			})
			if err != nil {
				if errors.Is(err, context.Canceled) || pctx.Err() != nil || lctx.Err() != nil {
					return
				}
				// or warn..
				// log.Error().Msgf("error publish notify-online message %v", err)
				return
			}
		}
	}()

	for anymsg := range subscription.Listen() {
		if pctx.Err() != nil {
			break
		}

		msg, ok := anymsg.(Row[DBusUnitStatus])
		if !ok {
			log.Error().Msgf("its not an event 😔 %T %+v", msg, msg)
			continue
		}

		err = h.sendToClient(pctx, c, msg)
		if err != nil && websocket.CloseStatus(err) == -1 {
			if pctx.Err() != nil {
				return
			}
			// log.Warn().Msgf("err listen to notifier event: %v %v", err, string(data))
			return
		}
	}

	// if we cannot publish anymore, return immediately
	if err := pctx.Err(); err != nil {
		return
	}

	// else, send goodbye message
	d := map[string]any{
		"evt_name": "listen-server-closed",
		"evt_ver":  0,
		"data":     "Server closed.",
	}

	err = h.sendToClient(pctx, c, d)
	if err != nil && websocket.CloseStatus(err) == -1 {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Err(err).Msgf("failed to send message")
		return
	}

	err = c.Close(websocket.StatusNormalClosure, "super duper X")
	if err != nil && websocket.CloseStatus(err) == -1 {
		log.Err(err).Msgf("failed to close websocket connection normally")
		return
	}

	log.Info().Msgf("websocket connection closed")
}

func (h *httpHandler) sendToClient(ctx context.Context, wsconn *websocket.Conn, msg any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = wsconn.Write(ctx, websocket.MessageText, payload)
	if err != nil {
		return err
	}

	return nil
}

type Command struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// {"name":"dbus", "data": {"action": "start", "unit": "nginx.service"} }

type DBusCommand struct {
	Action string `json:"action"`
	Unit   string `json:"unit"`
}

func (h *httpHandler) parseMessage(message []byte) error {
	var cmd Command
	err := json.Unmarshal(message, &cmd)
	if err != nil {
		return err
	}

	if cmd.Name == "dbus" {
		return h.handleDbusCommand(cmd.Data)
	}

	return nil
}

func (h *httpHandler) handleDbusCommand(data json.RawMessage) error {
	var cmd DBusCommand
	err := json.Unmarshal(data, &cmd)
	if err != nil {
		return err
	}

	switch cmd.Action {
	case "start":
		_, err = h.dbusConn.StartUnitContext(context.Background(), cmd.Unit, "replace", nil)
	case "stop":
		_, err = h.dbusConn.StopUnitContext(context.Background(), cmd.Unit, "replace", nil)
	default:
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}
