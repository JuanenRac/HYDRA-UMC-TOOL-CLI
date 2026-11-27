package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTheRefreshTokenIsNotSentOverPlainHTTPToAnotherHost(t *testing.T) {
	previous := activeTransport
	defer func() { activeTransport = previous }()
	activeTransport = transportSettings{}
	client := newClient(time.Second)
	// the address is never contacted: the refusal comes before the request
	_, _, err := postJSON(&client, "http://192.0.2.1:1", "/api/logout", map[string]string{"refreshToken": "sample-value"})
	if err == nil || !strings.Contains(err.Error(), "refusing to send") {
		t.Fatalf("expected the refusal, got %v", err)
	}
}

func TestARedirectDoesNotRepeatABodyOrACredential(t *testing.T) {
	var elsewhere int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { atomic.AddInt32(&elsewhere, 1) }))
	defer other.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer first.Close()
	client := newClient(5 * time.Second)

	status, _, err := postJSON(&client, first.URL, "/api/logout", map[string]string{"refreshToken": "sample-value"})
	if err != nil || status != http.StatusTemporaryRedirect {
		t.Fatalf("expected the 307 itself, got %d %v", status, err)
	}
	request, err := http.NewRequest(http.MethodGet, first.URL+"/api/robots", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer sample-value")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect || atomic.LoadInt32(&elsewhere) != 0 {
		t.Fatalf("a credential followed a redirect: status %d, other server hit %d time(s)", response.StatusCode, elsewhere)
	}

	plain, err := client.Get(first.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	plain.Body.Close()
	if plain.StatusCode != http.StatusOK || atomic.LoadInt32(&elsewhere) != 1 {
		t.Fatalf("a read without a credential no longer follows a redirect: %d", plain.StatusCode)
	}
}
