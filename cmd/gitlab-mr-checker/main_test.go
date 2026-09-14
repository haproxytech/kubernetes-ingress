package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GitLab pages notes newest first; the question posted long ago must still
// be found behind a page of newer notes.
func TestGetMergeRequestCommentsFollowsAllPages(t *testing.T) {
	var pagesServed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/projects/42/merge_requests/7/notes") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			t.Error("missing token header")
		}
		page := r.URL.Query().Get("page")
		pagesServed = append(pagesServed, page)
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "", "1":
			w.Header().Set("X-Next-Page", "2")
			_, _ = fmt.Fprint(w, `[{"id":3,"body":"newest"},{"id":2,"body":"newer"}]`)
		case "2":
			w.Header().Set("X-Next-Page", "")
			_, _ = fmt.Fprint(w, `[{"id":1,"body":"<!-- MR BACKPORT QUESTION -->"}]`)
		default:
			t.Errorf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	notes, err := getMergeRequestComments(srv.URL, "tok", "42", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 || notes[2].ID != 1 {
		t.Fatalf("want all 3 notes across pages, got %+v", notes)
	}
	if len(pagesServed) != 2 {
		t.Errorf("want exactly 2 page fetches, got %v", pagesServed)
	}
}

func TestGetMergeRequestCommentsReportsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"401 Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	if _, err := getMergeRequestComments(srv.URL, "bad", "42", 7); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want an error naming the status, got %v", err)
	}
}
