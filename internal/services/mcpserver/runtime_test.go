package mcpserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestRuntimeEndsEventStreamsBeforeWaitingForShutdown(t *testing.T) {
	for _, disable := range []bool{false, true} {
		t.Run(map[bool]string{false: "application close", true: "disable endpoint"}[disable], func(t *testing.T) {
			runtime, err := NewRuntime(testApplication(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Start(RuntimeConfig{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			instance := runtime.current
			t.Cleanup(func() { _ = instance.server.Close() })
			client := &http.Client{Timeout: 5 * time.Second}
			defer client.CloseIdleConnections()
			response, err := client.Get("http://" + instance.listener.Addr().String() + "/mcp")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
				t.Fatalf("expected an open event stream, got %s", response.Status)
			}
			streamDone := make(chan error, 1)
			go func() { _, err := io.Copy(io.Discard, response.Body); streamDone <- err }()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			started := time.Now()
			if disable {
				err = runtime.Start(RuntimeConfig{})
			} else {
				err = runtime.Close(ctx)
			}
			if err != nil {
				t.Fatalf("shutdown waited for a persistent client: %v", err)
			}
			if elapsed := time.Since(started); elapsed >= time.Second {
				t.Fatalf("shutdown took %v with an idle event stream", elapsed)
			}
			select {
			case err := <-streamDone:
				if err != nil {
					t.Fatalf("event stream did not end cleanly: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("event stream remained open after shutdown")
			}
		})
	}
}

func TestRuntimeHotStartsLoopbackStreamableHTTP(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	runtime, err := NewRuntime(testApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Start(RuntimeConfig{Enabled: true, Port: port}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = runtime.Close(ctx)
	})

	protocolClient, err := client.NewStreamableHttpClient("http://127.0.0.1:" + strconv.Itoa(port) + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protocolClient.Close() })
	if err := protocolClient.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{Name: "yotta-runtime-test", Version: "1"}
	if _, err := protocolClient.Initialize(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if tools, err := protocolClient.ListTools(context.Background(), mcp.ListToolsRequest{}); err != nil || len(tools.Tools) != 12 {
		t.Fatalf("tools = %#v, err = %v", tools, err)
	}
}

func TestRuntimePrepareRejectsOccupiedPortWithoutChangingSettings(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	runtime, err := NewRuntime(testApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	port := occupied.Addr().(*net.TCPAddr).Port
	if _, _, err := runtime.Prepare(RuntimeConfig{Enabled: true, Port: port}); err == nil {
		t.Fatal("occupied MCP port was accepted")
	}
}
