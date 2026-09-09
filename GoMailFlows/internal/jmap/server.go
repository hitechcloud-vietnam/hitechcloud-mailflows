package jmap

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"gorm.io/gorm"
)

// JMAP Server implements RFC 8620 (JMAP Core) and RFC 8621 (JMAP for Email)
// https://jmap.io/spec.html

type Server struct {
	cfg    *config.JMAPConfig
	db     *gorm.DB
	mux    *http.ServeMux
}

func NewServer(cfg *config.JMAPConfig, db *gorm.DB) *Server {
	s := &Server{
		cfg: cfg,
		db:  db,
		mux: http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// JMAP Session resource (RFC 8620 Section 2)
	s.mux.HandleFunc("/.well-known/jmap", s.handleSession)
	s.mux.HandleFunc("/jmap/session", s.handleSession)

	// JMAP API endpoint
	s.mux.HandleFunc("/jmap/api", s.handleAPI)

	// JMAP EventSource (Server-Sent Events for push)
	s.mux.HandleFunc("/jmap/eventsource", s.handleEventSource)

	// JMAP Upload endpoint
	s.mux.HandleFunc("/jmap/upload/", s.handleUpload)

	// JMAP Download endpoint
	s.mux.HandleFunc("/jmap/download/", s.handleDownload)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	log.Printf("JMAP server starting on %s", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	return server.ListenAndServe()
}

// ============================================================
// JMAP Session (RFC 8620 Section 2)
// ============================================================

type SessionResponse struct {
	Username        string                  `json:"username"`
	APIURL          string                  `json:"apiUrl"`
	DownloadURL     string                  `json:"downloadUrl"`
	UploadURL       string                  `json:"uploadUrl"`
	EventSourceURL  string                  `json:"eventSourceUrl"`
	State           string                  `json:"state"`
	Capabilities    map[string]interface{}  `json:"capabilities"`
	Accounts        map[string]JMAPAccount  `json:"accounts"`
	PrimaryAccounts map[string]string       `json:"primaryAccounts"`
}

type JMAPAccount struct {
	Name               string   `json:"name"`
	IsPersonal         bool     `json:"isPersonal"`
	IsReadOnly         bool     `json:"isReadOnly"`
	AccountCapabilities map[string]interface{} `json:"accountCapabilities"`
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Authenticate and get user from token
	// For now return a basic session

	baseURL := fmt.Sprintf("http://localhost:%d", s.cfg.Port)

	session := SessionResponse{
		Username:       "user@example.com",
		APIURL:         baseURL + "/jmap/api",
		DownloadURL:    baseURL + "/jmap/download/{blobId}/{type}/{name}",
		UploadURL:      baseURL + "/jmap/upload/{accountId}",
		EventSourceURL: baseURL + "/jmap/eventsource",
		State:          "0",
		Capabilities: map[string]interface{}{
			"urn:ietf:params:jmap:core": map[string]interface{}{
				"maxSizeUpload":           50000000,
				"maxConcurrentUpload":     4,
				"maxSizeRequest":          10000000,
				"maxConcurrentRequests":   4,
				"maxCallsInRequest":       16,
				"maxObjectsInGet":         500,
				"maxObjectsInSet":         500,
				"collationAlgorithms":     []string{"i;unicode-casemap"},
			},
			"urn:ietf:params:jmap:mail": map[string]interface{}{},
			"urn:ietf:params:jmap:websocket": map[string]interface{}{
				"url":         fmt.Sprintf("ws://localhost:%d/jmap/ws", s.cfg.Port),
				"supportsPush": true,
			},
		},
		Accounts: map[string]JMAPAccount{
			"account1": {
				Name:       "Default Account",
				IsPersonal: true,
				IsReadOnly: false,
				AccountCapabilities: map[string]interface{}{
					"urn:ietf:params:jmap:mail": map[string]interface{}{
						"maxMailboxesPerEmail":    10,
						"maxMailboxDepth":         5,
						"maxSizeMailboxName":      200,
						"maxSizeAttachmentsPerEmail": 50000000,
						"emailQuerySortOptions": []string{
							"receivedAt",
							"from",
							"to",
							"subject",
							"size",
							"header.x-spam-score",
						},
						"mayCreateTopLevelMailbox": true,
					},
				},
			},
		},
		PrimaryAccounts: map[string]string{
			"urn:ietf:params:jmap:mail": "account1",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// ============================================================
// JMAP API Endpoint (RFC 8620 Section 3.2)
// ============================================================

type JMAPRequest struct {
	Using       []string               `json:"using"`
	MethodCalls []MethodCall           `json:"methodCalls"`
	CreatedIDs  map[string]string      `json:"createdIds,omitempty"`
}

type MethodCall struct {
	Method string        `json:"method"`
	Args   interface{}   `json:"args"`
	CallID string        `json:"callId"`
}

type JMAPResponse struct {
	MethodResponses []MethodCall           `json:"methodResponses"`
	SessionState    string                 `json:"sessionState"`
	CreatedIDs      map[string]string      `json:"createdIds,omitempty"`
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JMAPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp := JMAPResponse{
		MethodResponses: make([]MethodCall, 0, len(req.MethodCalls)),
		SessionState:    "0",
		CreatedIDs:      req.CreatedIDs,
	}

	for _, call := range req.MethodCalls {
		result := s.processMethod(call)
		resp.MethodResponses = append(resp.MethodResponses, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) processMethod(call MethodCall) MethodCall {
	switch call.Method {
	case "Email/get":
		return s.emailGet(call)
	case "Email/query":
		return s.emailQuery(call)
	case "Email/set":
		return s.emailSet(call)
	case "Mailbox/get":
		return s.mailboxGet(call)
	case "Mailbox/query":
		return s.mailboxQuery(call)
	case "Mailbox/set":
		return s.mailboxSet(call)
	case "Thread/get":
		return s.threadGet(call)
	case "Identity/get":
		return s.identityGet(call)
	case "Submission/get":
		return s.submissionGet(call)
	case "Submission/set":
		return s.submissionSet(call)
	default:
		return MethodCall{
			Method: "error",
			CallID: call.CallID,
			Args: map[string]interface{}{
				"type": "unknownMethod",
			},
		}
	}
}

// ============================================================
// JMAP Email Methods (RFC 8621)
// ============================================================

func (s *Server) emailGet(call MethodCall) MethodCall {
	// TODO: Implement Email/get
	return MethodCall{
		Method: "Email/get",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"list":      []interface{}{},
			"notFound":  []interface{}{},
		},
	}
}

func (s *Server) emailQuery(call MethodCall) MethodCall {
	// TODO: Implement Email/query
	return MethodCall{
		Method: "Email/query",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"ids":       []string{},
			"total":     0,
		},
	}
}

func (s *Server) emailSet(call MethodCall) MethodCall {
	// TODO: Implement Email/set (create, update, destroy)
	return MethodCall{
		Method: "Email/set",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"created":   map[string]interface{}{},
			"updated":   map[string]interface{}{},
			"destroyed": []string{},
		},
	}
}

func (s *Server) mailboxGet(call MethodCall) MethodCall {
	// Return standard mailboxes
	return MethodCall{
		Method: "Mailbox/get",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"list": []map[string]interface{}{
				{
					"id":   "inbox",
					"name": "Inbox",
					"role": "inbox",
					"sortOrder": 1,
					"totalEmails": 0,
					"unreadEmails": 0,
				},
				{
					"id":   "sent",
					"name": "Sent",
					"role": "sent",
					"sortOrder": 2,
				},
				{
					"id":   "drafts",
					"name": "Drafts",
					"role": "drafts",
					"sortOrder": 3,
				},
				{
					"id":   "trash",
					"name": "Trash",
					"role": "trash",
					"sortOrder": 4,
				},
				{
					"id":   "spam",
					"name": "Spam",
					"role": "spam",
					"sortOrder": 5,
				},
			},
			"notFound": []string{},
		},
	}
}

func (s *Server) mailboxQuery(call MethodCall) MethodCall {
	return MethodCall{
		Method: "Mailbox/query",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"ids":       []string{"inbox", "sent", "drafts", "trash", "spam"},
			"total":     5,
		},
	}
}

func (s *Server) mailboxSet(call MethodCall) MethodCall {
	return MethodCall{
		Method: "Mailbox/set",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"created":   map[string]interface{}{},
			"updated":   map[string]interface{}{},
			"destroyed": []string{},
		},
	}
}

func (s *Server) threadGet(call MethodCall) MethodCall {
	return MethodCall{
		Method: "Thread/get",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"list":      []interface{}{},
			"notFound":  []interface{}{},
		},
	}
}

func (s *Server) identityGet(call MethodCall) MethodCall {
	return MethodCall{
		Method: "Identity/get",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"list": []map[string]interface{}{
				{
					"id":          "identity1",
					"name":        "Default",
					"email":       "user@example.com",
					"mayDelete":   false,
				},
			},
			"notFound": []string{},
		},
	}
}

func (s *Server) submissionGet(call MethodCall) MethodCall {
	return MethodCall{
		Method: "EmailSubmission/get",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"list":      []interface{}{},
			"notFound":  []interface{}{},
		},
	}
}

func (s *Server) submissionSet(call MethodCall) MethodCall {
	// TODO: Implement EmailSubmission/set (actually send the email)
	return MethodCall{
		Method: "EmailSubmission/set",
		CallID: call.CallID,
		Args: map[string]interface{}{
			"accountId": "account1",
			"state":     "0",
			"created":   map[string]interface{}{},
			"updated":   map[string]interface{}{},
			"destroyed": []string{},
		},
	}
}

// ============================================================
// JMAP EventSource (Server-Sent Events)
// ============================================================

func (s *Server) handleEventSource(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement SSE for real-time push notifications
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial state
	fmt.Fprintf(w, "id: 0\nevent: stateChange\ndata: {\"accountId\":\"account1\",\"type\":\"StateChange\",\"changed\":{\"account1\":{\"EmailDeliveryState\":\"0\",\"MailboxState\":\"0\",\"EmailState\":\"0\",\"ThreadState\":\"0\"}}}\n\n")
	flusher.Flush()

	// Keep connection open
	<-r.Context().Done()
}

// ============================================================
// Upload / Download
// ============================================================

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement blob upload
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"blobId":      "blob1",
		"type":        r.Header.Get("Content-Type"),
		"size":        0,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement blob download
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
