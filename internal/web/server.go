package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/keylime/keylime-mcp/internal/agent"
)

//go:embed templates/*
var templatesFS embed.FS

// Server represents the web server for the chat interface
type Server struct {
	agent       *agent.Agent
	templates   *template.Template
	eventChan   chan SSEvent
	pendingTool *agent.ToolRequest
	ctx         context.Context
}

// SSEvent represents a Server-Sent Event
type SSEvent struct {
	Event string
	Data  string
}

// NewServer creates a new web server instance
func NewServer(ag *agent.Agent, ctx context.Context) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		agent:     ag,
		templates: tmpl,
		eventChan: make(chan SSEvent, 100),
		ctx:       ctx,
	}, nil
}

// Start starts the web server
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("POST /tool/approve", s.handleToolApprove)
	mux.HandleFunc("POST /tool/deny", s.handleToolDeny)
	mux.HandleFunc("GET /events", s.handleSSE)
	mux.HandleFunc("POST /reset", s.handleReset)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-s.ctx.Done()
		log.Printf("[SERVER] Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("Starting web server on %s", addr)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Printf("[ERROR] Failed to render index: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	if message == "" {
		http.Error(w, "Message required", http.StatusBadRequest)
		return
	}

	log.Printf("[CHAT] User message: %s", message)

	s.send(SSEvent{
		Event: "user-message",
		Data:  s.renderMessage("user", message, "", nil),
	})

	go s.processMessage(message)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) processMessage(message string) {
	log.Printf("[AGENT] Processing message...")

	err := s.agent.SendMessage(s.ctx, message, s.handleMessage)

	if err != nil {
		log.Printf("[ERROR] Agent error: %v", err)
		s.send(SSEvent{
			Event: "error",
			Data:  s.renderMessage("error", fmt.Sprintf("Error: %v", err), "", nil),
		})
	}

	log.Printf("[AGENT] Message processing complete")
}

func (s *Server) handleMessage(msg agent.Message) {
	log.Printf("[AGENT] Callback: role=%s", msg.Role)

	switch msg.Role {
	case "tool_result":
		log.Printf("[TOOL] Result: %s", truncate(msg.Content, 100))
		s.send(SSEvent{
			Event: "tool-result",
			Data:  s.renderToolResult(msg.ToolID, msg.Content),
		})

	case "assistant":
		log.Printf("[AGENT] Assistant response: %s", truncate(msg.Content, 100))
		s.send(SSEvent{
			Event: "assistant-message",
			Data:  s.renderMessage("assistant", msg.Content, "", nil),
		})

	case "tool_request":
		log.Printf("[AGENT] Tool request: %s", msg.Tool.Name)
		s.pendingTool = msg.Tool
		s.send(SSEvent{
			Event: "tool-request",
			Data:  s.renderMessage("tool-request", "", msg.ToolID, msg.Tool),
		})
	}
}

func (s *Server) handleToolApprove(w http.ResponseWriter, r *http.Request) {
	if s.pendingTool == nil {
		http.Error(w, "No pending tool request", http.StatusBadRequest)
		return
	}

	log.Printf("[TOOL] Approved: %s", s.pendingTool.Name)
	go s.executeTool(s.pendingTool)
	s.pendingTool = nil

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleToolDeny(w http.ResponseWriter, r *http.Request) {
	tool := s.pendingTool
	s.pendingTool = nil
	if tool == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.send(SSEvent{
		Event: "tool-denied",
		Data:  s.renderMessage("system", "Tool execution denied by user.", "", nil),
	})
	err := s.agent.ToolDeny(s.ctx, tool, s.handleMessage)
	if err != nil {
		log.Printf("[ERROR] Tool deny response error: %v", err)
		s.send(SSEvent{
			Event: "error",
			Data:  s.renderMessage("error", fmt.Sprintf("Error: %v", err), "", nil),
		})
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) executeTool(tool *agent.ToolRequest) {
	log.Printf("[TOOL] Executing: %s", tool.Name)

	s.send(SSEvent{
		Event: "tool-executing",
		Data:  tool.ID,
	})

	err := s.agent.ExecuteTool(s.ctx, tool, s.handleMessage)

	if err != nil {
		log.Printf("[ERROR] Tool execution error: %v", err)
		s.send(SSEvent{
			Event: "error",
			Data:  s.renderMessage("error", fmt.Sprintf("Error: %v", err), "", nil),
		})
	}

	log.Printf("[TOOL] Execution complete")
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	log.Printf("[CHAT] Reset conversation")
	s.agent.Reset()
	s.pendingTool = nil

	s.send(SSEvent{
		Event: "reset",
		Data:  "",
	})

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	log.Printf("[SSE] Client connected")

	fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			log.Printf("[SSE] Client disconnected")
			return
		case event := <-s.eventChan:
			data := strings.ReplaceAll(event.Data, "\n", "\\n")
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Event, data)
			flusher.Flush()
		case <-time.After(30 * time.Second):
			fmt.Fprintf(w, "event: ping\ndata: keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) send(event SSEvent) {
	select {
	case s.eventChan <- event:
		log.Printf("[SSE] Sent: %s", event.Event)
	default:
		log.Printf("[SSE] Channel full, dropping event: %s", event.Event)
	}
}

func (s *Server) renderMessage(role, content, toolID string, tool *agent.ToolRequest) string {
	data := map[string]interface{}{
		"Role":    role,
		"Content": content,
		"ToolID":  toolID,
	}

	if tool != nil {
		argsJSON, _ := json.MarshalIndent(tool.Arguments, "", "  ")
		data["ToolName"] = tool.Name
		data["ToolArgs"] = string(argsJSON)
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "message.html", data); err != nil {
		log.Printf("[ERROR] Template error: %v", err)
		return fmt.Sprintf("<div class=\"message message-system\"><div class=\"message-content\"><div class=\"message-text\">Render error: %v</div></div></div>", err)
	}

	return buf.String()
}

func (s *Server) renderToolResult(toolID, content string) string {
	data := map[string]interface{}{
		"ToolID":  toolID,
		"Content": content,
	}

	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "tool-result.html", data); err != nil {
		log.Printf("[ERROR] Template error: %v", err)
		return fmt.Sprintf("<div class=\"tool-result\">Render error: %v</div>", err)
	}

	return buf.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
