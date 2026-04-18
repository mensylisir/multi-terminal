package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
)

type CommandLog struct {
	Timestamp   string `json:"timestamp"`
	UserID      string `json:"userId"`
	SessionID   uint32 `json:"sessionId"`
	HostID      string `json:"hostId"`
	CommandType string `json:"commandType"`
	RawInput    string `json:"rawInput"`
	CleanInput  string `json:"cleanInput"`
	RiskLevel   string `json:"riskLevel"`
}

type StreamLog struct {
	Timestamp string `json:"timestamp"`
	SessionID uint32 `json:"sessionId"`
	Data      []byte `json:"data"`
}

type Service struct {
	filter     *SensitiveFilter
	encoder    *zstd.Encoder
	logDir     string
	bufferSize int
}

func NewService(logDir string) (*Service, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create audit log dir: %w", err)
	}

	// Create Zstd encoder
	encoder, err := zstd.NewWriter(nil, zstd.WithCompressionLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("create zstd encoder: %w", err)
	}

	return &Service{
		filter:     NewSensitiveFilter(),
		encoder:    encoder,
		logDir:     logDir,
		bufferSize: 1024 * 1024, // 1MB buffer
	}, nil
}

// LogCommand creates a structured JSON command log entry
func (s *Service) LogCommand(userID string, sessionID uint32, hostID, rawInput string, riskLevel string) (*CommandLog, error) {
	// Filter sensitive data
	cleanInput := s.filter.Filter(rawInput)

	log := &CommandLog{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		UserID:       userID,
		SessionID:    sessionID,
		HostID:       hostID,
		CommandType:  "shell_input",
		RawInput:     rawInput,
		CleanInput:   cleanInput,
		RiskLevel:    riskLevel,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(log)
	if err != nil {
		return nil, fmt.Errorf("marshal command log: %w", err)
	}

	// Write to file (in production, this would go to S3/MinIO)
	filename := fmt.Sprintf("%s/command_%s_%d.json", s.logDir, time.Now().Format("20060102_150405"), sessionID)
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return log, fmt.Errorf("write command log: %w", err)
	}

	return log, nil
}

// CompressData compresses raw stream data using Zstd
func (s *Service) CompressData(data []byte) ([]byte, error) {
	return s.encoder.EncodeAll(data, nil), nil
}

// Close closes the audit service
func (s *Service) Close() error {
	s.encoder.Close()
	return nil
}