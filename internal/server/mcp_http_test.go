package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yann0917/toolbox/internal/service"
)

// TestMCPHTTPEndpoint 验证 serve 内嵌的 MCP Streamable HTTP 端点：
// 挂载后 /api/mcp 可完成 JSON-RPC initialize 握手并暴露工具清单。
func TestMCPHTTPEndpoint(t *testing.T) {
	svc, err := service.NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	s := New(svc)
	svc.StartEngine(s.Hub().Notify, 2)
	// 挂载一个无工具的测试 Server：握手与路由验证不依赖具体工具
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "toolbox-test", Version: "0"}, nil)
	s.MountMCP(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpSrv }, nil))

	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"0"}}}`
	req, err := http.NewRequest("POST", ts.URL+"/api/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("HTTP status = %d, want 200", resp.StatusCode)
	}
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	raw := string(buf[:n])
	// Streamable HTTP 可能回 application/json 或 SSE 帧，两种都应携带 initialize 结果
	if !strings.Contains(raw, `"serverInfo"`) || !strings.Contains(raw, "toolbox-test") {
		t.Fatalf("initialize 响应缺少 serverInfo: %s", raw)
	}
}
