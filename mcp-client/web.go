package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Message struct {
	Role    string
	Content string
	Time    string
}

type WebServer struct {
	agent    *Agent
	router   *chi.Mux
	messages []Message
	mu       sync.RWMutex
	tmpl     *template.Template
}

func NewWebServer(agent *Agent) (*WebServer, error) {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	ws := &WebServer{
		agent:    agent,
		messages: []Message{},
		tmpl:     tmpl,
	}

	agent.SetOutputHandler(func(message string) {
		ws.addMessage("assistant", message)
	})

	ws.setupRouter()
	return ws, nil
}

func (ws *WebServer) setupRouter() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", ws.handleIndex)
	r.Post("/send", ws.handleSend)

	ws.router = r
}

func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	data := struct {
		Messages []Message
	}{
		Messages: ws.messages,
	}
	ws.mu.RUnlock()

	if err := ws.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (ws *WebServer) handleSend(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	ws.addMessage("user", userMessage)

	ctx := context.Background()
	if err := ws.agent.ProcessQuery(ctx, userMessage); err != nil {
		log.Printf("Error processing query: %v", err)
		ws.addMessage("assistant", fmt.Sprintf("Error: %v", err))
	}

	ws.renderMessages(w)
}

func (ws *WebServer) addMessage(role, content string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.messages = append(ws.messages, Message{
		Role:    role,
		Content: content,
		Time:    time.Now().Format("15:04:05"),
	})
}

func (ws *WebServer) renderMessages(w http.ResponseWriter) {
	ws.mu.RLock()
	data := struct {
		Messages []Message
	}{
		Messages: ws.messages,
	}
	ws.mu.RUnlock()

	if err := ws.tmpl.ExecuteTemplate(w, "messages.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func (ws *WebServer) Start(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: ws.router,
	}

	go func() {
		url := fmt.Sprintf("http://localhost%s", addr)
		log.Printf("Starting web server at %s", url)
		log.Printf("Click here to open: %s", url)

		if err := openBrowser(url); err != nil {
			log.Printf("Could not open browser automatically: %v", err)
			log.Printf("Please open your browser manually and go to: %s", url)
		}
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Web server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down web server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
