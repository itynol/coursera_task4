package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	SuccessAccessToken = "success_token"
)

const (
	OneUserSuccess = `[{"Id": 1, "Name": "Success", "Age": 42, "About": "Smt", "Gender": "male"}]`
	ResponseEmpty  = ``
)

func BabbleServer(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Accesstoken")
	name := r.FormValue("query")
	if key != SuccessAccessToken {
		w.WriteHeader(http.StatusUnauthorized)
	}
	switch name {
	case "Success":
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, OneUserSuccess)
	case "Fatal":
		w.WriteHeader(http.StatusInternalServerError)
	case "BadRequest":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "")
	case "ErrorBadOrderField":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error": "ErrorBadOrderField"}`)
	case "Unknown":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error": "Unknown"}`)
	case "UnparsedResult":
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, ResponseEmpty)
	case "LargeResponse":
		w.WriteHeader(http.StatusOK)
		var users []User
		for i := 0; i <= 25; i++ {
			users = append(users, User{
				Id: i,
			})
		}
		data, _ := json.Marshal(users)
		io.WriteString(w, string(data))
	case "Timeout":
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusRequestTimeout)
	}
}

func NewSearchClient(act, url string) SearchClient {
	return SearchClient{
		AccessToken: act,
		URL:         url,
	}
}

func NewSearchRequest(limit, offset, orderby int, query, orderfield string) SearchRequest {
	return SearchRequest{
		Limit:      limit,
		Offset:     offset,
		Query:      query,
		OrderField: orderfield,
		OrderBy:    orderby,
	}
}

func TestSearchClient_FindUsers(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(BabbleServer))
	defer testServer.Close()
	sc := NewSearchClient(SuccessAccessToken, testServer.URL)
	t.Run("success_way", func(t *testing.T) {
		req := NewSearchRequest(24, 1, 0, `Success`, "")
		resp, err := sc.FindUsers(req)
		expectedResp := &SearchResponse{
			Users: []User{
				{
					Id:     1,
					Name:   "Success",
					Age:    42,
					About:  "Smt",
					Gender: "male",
				},
			},
			NextPage: false,
		}
		customEqual(t, expectedResp, resp)
		errNil(t, err, "Error must be nil")
	})
	t.Run("negative_limit", func(t *testing.T) {
		req := NewSearchRequest(-1, 1, 0, "", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "limit must be > 0")
	})
	t.Run("negative_offset", func(t *testing.T) {
		req := NewSearchRequest(1, -1, 0, "", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "offset must be > 0")
	})
	t.Run("search_server_fatal_error", func(t *testing.T) {
		req := NewSearchRequest(1, 1, 0, "Fatal", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "SearchServer fatal error")
	})
	t.Run("bad_request_unparsed_err", func(t *testing.T) {
		req := NewSearchRequest(1, 1, 0, "BadRequest", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "cant unpack error json: unexpected end of JSON input")
	})
	t.Run("bad_request_unparsed_err", func(t *testing.T) {
		req := NewSearchRequest(1, 1, 0, "ErrorBadOrderField", "high")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "OrderFeld high invalid")
	})
	t.Run("bad_request_unparsed_err", func(t *testing.T) {
		req := NewSearchRequest(1, 1, 0, "Unknown", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "unknown bad request error: Unknown")
	})
	t.Run("unparsed_results", func(t *testing.T) {
		req := NewSearchRequest(1, 1, 0, "UnparsedResult", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "cant unpack result json: unexpected end of JSON input")
	})
	t.Run("large_response", func(t *testing.T) {
		req := NewSearchRequest(30, 0, 0, "LargeResponse", "")
		resp, err := sc.FindUsers(req)
		errNil(t, err, "Error must be nil")
		if !resp.NextPage {
			t.Error("NextPage must be true")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		req := NewSearchRequest(30, 0, 0, "Timeout", "")
		resp, err := sc.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), "timeout for limit=26&offset=0&order_by=0&order_field=&query=Timeout")
	})
	t.Run("unknown_error", func(t *testing.T) {
		rm := NewSearchClient(SuccessAccessToken, "")
		req := NewSearchRequest(30, 0, 0, "UnknownError", "")
		resp, err := rm.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		if !strings.Contains(err.Error(), `unknown error`) {
			t.Error("Error must contains: unknown error")
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		rm := NewSearchClient("", testServer.URL)
		req := NewSearchRequest(1, 0, 0, "Unauthorized", "")
		resp, err := rm.FindUsers(req)
		respNil(t, resp, "Resp must be nil")
		customEqual(t, err.Error(), `Bad AccessToken`)
	})
}

func respNil(t * testing.T, c *SearchResponse, errMsg string) {
	if c != nil {
		t.Error(errMsg)
	}
}

func errNil(t * testing.T, c error, errMsg string) {
	if c != nil {
		t.Error(errMsg)
	}
}

func customEqual(t * testing.T, c, v interface {}) {
	if !reflect.DeepEqual(c, v) {
		t.Error("Not equal")
	}
}