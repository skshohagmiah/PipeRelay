package delivery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shohag/piperelay/internal/signing"
)

type SendResult struct {
	StatusCode   int
	ResponseBody string
	LatencyMs    int64
	Error        string
}

type Sender struct {
	client *http.Client
}

func NewSender(timeout time.Duration) *Sender {
	return &Sender{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *Sender) Send(ctx context.Context, url, secret, messageID string, payload []byte) *SendResult {
	start := time.Now()

	signature, timestamp := signing.Sign(secret, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return &SendResult{
			Error:     fmt.Sprintf("failed to create request: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PipeRelay/1.0")
	req.Header.Set("X-PipeRelay-ID", messageID)
	req.Header.Set("X-PipeRelay-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-PipeRelay-Signature", signature)

	resp, err := s.client.Do(req)
	if err != nil {
		return &SendResult{
			Error:     fmt.Sprintf("request failed: %v", err),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return &SendResult{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(body),
		LatencyMs:    time.Since(start).Milliseconds(),
	}
}
