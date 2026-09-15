package objectstorage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestTOS(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cli, err := New(Config{
		Provider: "tos", Endpoint: srv.URL, Region: "cn-test",
		Bucket: "bkt", AccessKey: "ak", SecretKey: "sk", Prefix: "toolbox",
		InsecureSkipTLSVerify: true,
	})
	if err != nil {
		t.Fatalf("New() err = %v", err)
	}
	return cli, srv
}

func TestNewConfigValidation(t *testing.T) {
	if cli, err := New(Config{}); cli != nil || err != nil {
		t.Fatalf("未启用配置应返回 (nil, nil)，得到 (%v, %v)", cli, err)
	}
	if _, err := New(Config{Provider: "tos"}); err == nil {
		t.Fatal("provider=tos 但配置不全，应报错")
	}
	if _, err := New(Config{Provider: "s3", Endpoint: "e", Region: "r", Bucket: "b", AccessKey: "a", SecretKey: "s"}); err == nil {
		t.Fatal("s3 通道未实现，应报「暂未支持」")
	}
}

func TestTOSPutPresignRoundTrip(t *testing.T) {
	var gotPath, gotCT, gotBody string
	cli, _ := newTestTOS(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("x-tos-request-id", "test-req")
		w.WriteHeader(200)
	})
	key := ObjectKey("20260916/test.mp3")
	if !strings.HasPrefix(key, "20260916/") || !strings.HasSuffix(key, ".mp3") {
		t.Fatalf("ObjectKey = %q", key)
	}
	if err := cli.Put(context.Background(), key, ContentTypeByExt(key), strings.NewReader("hello-audio"), 11); err != nil {
		t.Fatalf("Put() err = %v", err)
	}
	if !strings.Contains(gotPath, "bkt") || !strings.HasSuffix(gotPath, key) {
		t.Fatalf("上传路径 = %q, 期望含桶与前缀 key", gotPath)
	}
	if gotCT != "audio/mpeg" {
		t.Fatalf("Content-Type = %q", gotCT)
	}
	if gotBody != "hello-audio" {
		t.Fatalf("上传内容 = %q", gotBody)
	}

	signed, err := cli.PresignGet(key, 72*time.Hour)
	if err != nil {
		t.Fatalf("PresignGet() err = %v", err)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("签名 URL 无法解析: %v", err)
	}
	if !strings.HasSuffix(u.Path, key) {
		t.Fatalf("签名 URL 路径 = %q, 期望含 key", u.Path)
	}
	if u.RawQuery == "" {
		t.Fatal("签名 URL 缺少签名查询参数")
	}
}

func TestTOSApplyLifecycleMerge(t *testing.T) {
	var putRules json.RawMessage
	var putCalls int
	cli, _ := newTestTOS(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// 返回一条用户自建规则 + 本工具旧规则（days=1）
			w.Write([]byte(`{"Rules":[{"ID":"user-keep","Prefix":"keep","Status":"Enabled","Expiration":{"Days":90}},{"ID":"toolbox-auto-clean","Prefix":"toolbox","Status":"Enabled","Expiration":{"Days":1}}]}`))
		case http.MethodPut:
			putCalls++
			_ = json.NewDecoder(r.Body).Decode(&putRules)
			w.WriteHeader(200)
		}
	})
	msg, err := cli.ApplyLifecycle(context.Background(), 3)
	if err != nil {
		t.Fatalf("ApplyLifecycle() err = %v", err)
	}
	if putCalls != 1 || !strings.Contains(msg, "3 天") {
		t.Fatalf("putCalls=%d msg=%q", putCalls, msg)
	}
	var wrapper struct {
		Rules []struct {
			ID         string `json:"ID"`
			Prefix     string `json:"Prefix"`
			Expiration *struct {
				Days int `json:"Days"`
			} `json:"Expiration"`
		} `json:"Rules"`
	}
	if err := json.Unmarshal(putRules, &wrapper); err != nil {
		t.Fatalf("解析 PUT 规则失败: %v (%s)", err, putRules)
	}
	rules := wrapper.Rules
	if len(rules) != 2 {
		t.Fatalf("规则数 = %d, 期望保留用户规则 + 本工具新规则: %s", len(rules), putRules)
	}
	if rules[0].ID != "user-keep" || rules[0].Expiration.Days != 90 {
		t.Fatalf("用户规则被改动: %+v", rules[0])
	}
	if rules[1].ID != lifecycleRuleID || rules[1].Prefix != "toolbox" || rules[1].Expiration.Days != 3 {
		t.Fatalf("本工具规则未按 3 天更新: %+v", rules[1])
	}
}

func TestTOSApplyLifecycleFromEmpty(t *testing.T) {
	var putRules json.RawMessage
	cli, _ := newTestTOS(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound) // 桶上无规则：TOS 404 语义
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&putRules)
		w.WriteHeader(200)
	})
	if _, err := cli.ApplyLifecycle(context.Background(), 3); err != nil {
		t.Fatalf("ApplyLifecycle() err = %v", err)
	}
	if !strings.Contains(string(putRules), `"Days":3`) || !strings.Contains(string(putRules), lifecycleRuleID) {
		t.Fatalf("空规则集应只写入本工具 3 天规则: %s", putRules)
	}
}

func TestTOSPingMapsErrors(t *testing.T) {
	cli, _ := newTestTOS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"Code":"AccessDenied"}`))
	})
	err := cli.Ping(context.Background())
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("Ping() err = %v, 期望中文提示含 403", err)
	}
}
