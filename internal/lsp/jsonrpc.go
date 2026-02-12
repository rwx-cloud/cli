package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type jsonrpcConn struct {
	writer io.Writer
	reader *bufio.Reader
	mu     sync.Mutex // protects writer
	nextID atomic.Int64

	pending   map[int64]chan jsonrpcMessage
	pendingMu sync.Mutex
}

func newJSONRPCConn(r io.Reader, w io.Writer) *jsonrpcConn {
	return &jsonrpcConn{
		writer:  w,
		reader:  bufio.NewReader(r),
		pending: make(map[int64]chan jsonrpcMessage),
	}
}

func (c *jsonrpcConn) readLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		msg, err := c.readMessage()
		if err != nil {
			// Connection closed or context cancelled — close all pending channels
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			return
		}

		if msg.ID != nil && msg.Method == "" {
			// This is a response
			c.pendingMu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.pendingMu.Unlock()
			if ok {
				ch <- msg
			}
		}
		// Notifications from server (e.g. textDocument/publishDiagnostics) are ignored;
		// we use pull diagnostics instead.
	}
}

func (c *jsonrpcConn) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	ch := make(chan jsonrpcMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	if err := c.send(method, &id, params); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed while waiting for response to %s", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error on %s: %s", method, string(resp.Error))
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *jsonrpcConn) notify(method string, params any) error {
	return c.send(method, nil, params)
}

func (c *jsonrpcConn) send(method string, id *int64, params any) error {
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
	}

	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshaling params for %s: %w", method, err)
		}
		msg.Params = raw
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message for %s: %w", method, err)
	}

	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = io.WriteString(c.writer, frame)
	return err
}

func (c *jsonrpcConn) readMessage() (jsonrpcMessage, error) {
	contentLength := -1

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return jsonrpcMessage{}, err
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(val)
			if err != nil {
				return jsonrpcMessage{}, fmt.Errorf("invalid Content-Length: %s", val)
			}
		}
	}

	if contentLength < 0 {
		return jsonrpcMessage{}, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return jsonrpcMessage{}, fmt.Errorf("reading message body: %w", err)
	}

	var msg jsonrpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return jsonrpcMessage{}, fmt.Errorf("unmarshaling message: %w", err)
	}

	return msg, nil
}
