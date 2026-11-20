package zotero

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// listenSilently returns the address of a socket that completes the TCP
// handshake (the kernel's backlog does that) but never reads or answers.
func listenSilently(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return "http://" + ln.Addr().String()
}

// listenAndHangUp returns the address of a socket that accepts each connection
// and closes it without a response.
func listenAndHangUp(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return "http://" + ln.Addr().String()
}

// The three ways the ping can fail to find Zotero, from #151, each against a
// real stand-in. None of them is "Zotero not running" unless nothing accepted
// the connection, and none of them is "Zotero running".
func TestCheckHealth_PingClassification(t *testing.T) {
	refused := httptest.NewServer(http.NewServeMux())
	refusedURL := refused.URL
	refused.Close()

	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()

	// A 200 to the ping, but without the header Zotero puts on every response.
	unsigned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Zotero is running"))
	}))
	defer unsigned.Close()

	for _, c := range []struct {
		name         string
		url          string
		reachable    bool
		unresponsive bool
		reason       string
	}{
		{"nothing listening", refusedURL, false, false, "nothing accepted a connection"},
		{"accepts, never answers", listenSilently(t), false, true, "something is listening but did not answer"},
		{"accepts, hangs up", listenAndHangUp(t), false, true, "something is listening but did not answer"},
		{"unrelated server, 404", notFound.URL, true, false, "something other than Zotero is answering"},
		{"200 without X-Zotero-Version", unsigned.URL, true, false, "something other than Zotero is answering"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := New(c.url, WithTimeout(200*time.Millisecond)).CheckHealth(context.Background())
			if h.ZoteroRunning || h.Ready() {
				t.Fatalf("running=%v ready=%v, want both false: %+v", h.ZoteroRunning, h.Ready(), h)
			}
			if h.Reachable != c.reachable || h.Unresponsive != c.unresponsive {
				t.Errorf("reachable=%v unresponsive=%v, want %v/%v", h.Reachable, h.Unresponsive, c.reachable, c.unresponsive)
			}
			for _, capability := range h.Capabilities() {
				if capability.Name == CapabilityRead && capability.Reason != c.reason {
					t.Errorf("read reason = %q, want %q", capability.Reason, c.reason)
				}
			}
		})
	}
}

// X-Zotero-Version identifies Zotero whatever the status: Zotero puts it on
// every response, so a ping it answers with an error is still Zotero answering.
func TestCheckHealth_ZoteroHeaderOnErrorStatusIsZotero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Zotero-Version", "9.9.9")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := New(srv.URL).CheckHealth(context.Background())
	if !h.ZoteroRunning || h.ZoteroVersion != "9.9.9" {
		t.Errorf("running=%v version=%q, want true/9.9.9", h.ZoteroRunning, h.ZoteroVersion)
	}
}

// The client's own timeout is not the caller's deadline. Something accepted the
// connection and did not answer: a transport fault, not a cancellation.
func TestClientTimeout_IsTransport(t *testing.T) {
	_, _, err := New(listenSilently(t), WithTimeout(100*time.Millisecond)).Items(context.Background(), UserLibrary(), ItemsOptions{})
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("err = %v, want ErrTransport", err)
	}
	if errors.Is(err, ErrZoteroDown) {
		t.Fatalf("timeout misreported as ErrZoteroDown: %v", err)
	}
}
