package service

import (
	"bytes"
	"context"
	"crypto/subtle"
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
	token  string
}

func New(token string) *Service {
	return &Service{
		engine: &engine.SingBoxEngine{},
		token:  token,
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
	if !s.authorize(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": s.engine.Status()})
}

func (s *Service) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
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

	// 启动前先停止已运行实例并清理可能残留的 TUN 适配器
	_ = s.engine.Stop()
	_ = sys.CleanupWindowsTUN()

	if err := s.engine.Start(req.ConfigJSON); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.engine.SetCurrentSubID(req.SubID)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = s.engine.Stop()
	_ = sys.CleanupWindowsTUN()
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleCurrentSubID(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"sub_id": s.engine.GetCurrentSubID()})
}

func (s *Service) authorize(w http.ResponseWriter, r *http.Request) bool {
	if s.token == "" {
		http.Error(w, "service token not configured", http.StatusInternalServerError)
		return false
	}
	got := r.Header.Get("X-Zem-Service-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// Client 用于 GUI 连接后台 Service
type Client struct {
	baseURL string
	client  *http.Client
	token   string
}

func NewClient(port int, token string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client:  &http.Client{Timeout: 10 * time.Second},
		token:   token,
	}
}

func (c *Client) reachable() bool {
	_, err := c.Status()
	return err == nil
}

func (c *Client) Status() (string, error) {
	req, err := c.newRequest(http.MethodGet, "/api/status", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("service status failed: %s", string(data))
	}
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
	req, err := c.newRequest(http.MethodPost, "/api/connect", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
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
	req, err := c.newRequest(http.MethodPost, "/api/disconnect", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
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
	req, err := c.newRequest(http.MethodGet, "/api/current-sub-id", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("service current-sub-id failed: %s", string(data))
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["sub_id"], nil
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Zem-Service-Token", c.token)
	return req, nil
}
