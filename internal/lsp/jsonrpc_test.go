package lsp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJSONRPC_RoundTrip(t *testing.T) {
	// Create two pipe pairs: client writes to serverReader, server writes to clientReader
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	client := newJSONRPCConn(clientReader, clientWriter)
	server := newJSONRPCConn(serverReader, serverWriter)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go client.readLoop(ctx)

	// Server goroutine: read one request and respond
	go func() {
		msg, err := server.readMessage()
		if err != nil {
			return
		}
		// Send response
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"status":"ok"}}`, *msg.ID)
		frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(resp), resp)
		_, _ = io.WriteString(serverWriter, frame)
	}()

	result, err := client.request(ctx, "test/method", map[string]string{"key": "value"})
	require.NoError(t, err)
	require.Contains(t, string(result), `"status":"ok"`)
}

func TestJSONRPC_ReadMessage_ParsesContentLength(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":null}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	conn := newJSONRPCConn(bytes.NewReader([]byte(frame)), io.Discard)
	msg, err := conn.readMessage()
	require.NoError(t, err)
	require.NotNil(t, msg.ID)
	require.Equal(t, int64(1), *msg.ID)
}

func TestJSONRPC_RequestTimeout(t *testing.T) {
	// clientReader: never receives data so readLoop blocks
	// serverReader: drains writes from send() so it doesn't block
	clientReader, _ := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// Drain writes so send() doesn't block on the pipe
	go func() { _, _ = io.Copy(io.Discard, serverReader) }()

	conn := newJSONRPCConn(clientReader, clientWriter)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go conn.readLoop(ctx)

	_, err := conn.request(ctx, "test/timeout", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestJSONRPC_Notify_NoResponse(t *testing.T) {
	var buf bytes.Buffer
	conn := newJSONRPCConn(bytes.NewReader(nil), &buf)

	err := conn.notify("test/notification", map[string]int{"value": 42})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Content-Length:")
	require.Contains(t, buf.String(), `"method":"test/notification"`)
	// Notifications should not have an id
	require.NotContains(t, buf.String(), `"id"`)
}
