package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

func ListenFromWebsocket(ctx context.Context, url string) <-chan json.RawMessage {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Connect to WebSocket server
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		log.Fatal().Msgf("dial error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	fmt.Println("Connected to server")

	// Send initial message
	// err = conn.Write(ctx, websocket.MessageText, []byte("Hello from client"))
	// if err != nil {
	// 	log.Fatal().Msgf("write error: %v", err)
	// }

	// Read messages in goroutine
	msgChan := make(chan json.RawMessage)
	go func() {
		defer close(msgChan)

		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				log.Error().Msgf("read error: %v", err)
				cancel()
				return
			}

			msgChan <- json.RawMessage(msg)
		}
	}()

	return msgChan
}
