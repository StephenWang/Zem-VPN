package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"zem/internal/engine"
	"zem/internal/sys"
)

// Service 以 HTTP API 形式在后台运行 sing-box 引擎
type Service struct {
	engine *engine.SingBoxEngine
	server *http.Server
	mu     sync.Mutex
}

func New() *Service {
	return &Service{
		engine: &engine.SingBoxEngine{},
	}
}

func (s *Service) Start(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return fmt.Errorf("service already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/connect", s.handleConnect)
	mux.HandleFunc("/api/disconnect", s.handleDisconnect)
	mux.HandleFunc("/api/current-sub-id", s.handleCurrentSubID)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("service server error:", err)
		}
	}()
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return
	}

	s.engine.Stop()
	_ = sys.CleanupWindowsTUN()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	s.server = nil
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": s.engine.Status()})
}

func (s *Service) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ConfigJSON string `json:"config_json"`
		SubID      string `json:"sub_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ConfigJSON == "" {
		http.Error(w, "config_json required", http.StatusBadRequest)
		return
	}

	// 启动前先清理可能残留的 TUN 适配器
	_ = sys.CleanupWindowsTUN()

	if err := s.engine.Start(req.ConfigJSON); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.engine.SetCurrentSubID(req.SubID)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = s.engine.Stop()
	_ = sys.CleanupWindowsTUN()
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleCurrentSubID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"sub_id": s.engine.GetCurrentSubID()})
}

// Client 用于 GUI 连接后台 Service
type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) reachable() bool {
	resp, err := c.client.Get(c.baseURL + "/api/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) Status() (string, error) {
	resp, err := c.client.Get(c.baseURL + "/api/status")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["status"], nil
}

func (c *Client) Connect(configJSON, subID string) error {
	body, _ := json.Marshal(map[string]string{
		"config_json": configJSON,
		"sub_id":      subID,
	})
	resp, err := c.client.Post(c.baseURL+"/api/connect", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("service connect failed: %s", string(data))
	}
	return nil
}

func (c *Client) Disconnect() error {
	resp, err := c.client.Post(c.baseURL+"/api/disconnect", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("service disconnect failed: %s", string(data))
	}
	return nil
}

func (c *Client) GetCurrentSubID() (string, error) {
	resp, err := c.client.Get(c.baseURL + "/api/current-sub-id")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["sub_id"], nil
}
