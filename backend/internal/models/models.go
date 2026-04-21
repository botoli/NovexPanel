package models

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Email        string `gorm:"uniqueIndex;size:190;not null" json:"email"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AgentToken struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	Name        *string    `gorm:"type:text" json:"name"`
	TokenHash   string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	TokenPrefix string     `gorm:"size:24;not null" json:"token_prefix"`
	Revoked     bool       `gorm:"index;default:false" json:"revoked"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time
}

type Server struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         uint           `gorm:"index;not null" json:"user_id"`
	TokenID        uint           `gorm:"uniqueIndex;not null" json:"token_id"`
	Name           string         `gorm:"size:120" json:"name"`
	IP             string         `gorm:"size:80" json:"ip"`
	Online         bool           `gorm:"index;default:false" json:"online"`
	ConnectedAt    *time.Time     `json:"connected_at"`
	DisconnectedAt *time.Time     `json:"disconnected_at"`
	LastMetrics    datatypes.JSON `json:"last_metrics"`
	Deploys        []Deploy       `gorm:"foreignKey:ServerID" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type MetricPoint struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ServerID       uint           `gorm:"index:idx_metric_server_ts,priority:1;not null" json:"server_id"`
	Timestamp      time.Time      `gorm:"index:idx_metric_server_ts,priority:2;not null" json:"timestamp"`
	CPUUsage       float64        `json:"cpu_usage"`
	RAMPercent     float64        `json:"ram_percent"`
	DiskPercent    float64        `json:"disk_percent"`
	DiskReadBytes  float64        `gorm:"not null;default:0" json:"disk_read_bytes"`
	DiskWriteBytes float64        `gorm:"not null;default:0" json:"disk_write_bytes"`
	NetworkRXBytes float64        `gorm:"not null;default:0" json:"network_rx_bytes"`
	NetworkTXBytes float64        `gorm:"not null;default:0" json:"network_tx_bytes"`
	Raw            datatypes.JSON `json:"raw"`
	CreatedAt      time.Time
}

type Deploy struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"user_id"`
	ServerID     uint       `gorm:"index;not null" json:"server_id"`
	Branch       string     `gorm:"size:120;default:main" json:"branch"`
	BuildCommand string     `gorm:"size:512" json:"build_command"`
	OutputDir    string     `gorm:"size:512" json:"output_dir"`
	Subdirectory string     `gorm:"type:varchar(255);default:''" json:"subdirectory"`
	Port         int        `gorm:"default:0" json:"port"`
	Source       string     `gorm:"size:20;not null" json:"source"`
	Status       string     `gorm:"size:20;index;not null" json:"status"`
	ProjectType  string     `gorm:"size:20" json:"project_type"`
	RepoURL      string     `gorm:"size:512" json:"repo_url"`
	URL          string     `gorm:"size:512" json:"url"`
	DeployLog    string     `gorm:"type:text" json:"deploy_log"`
	ResultURL    string     `gorm:"size:512" json:"result_url"`
	ErrorMessage string     `gorm:"size:1024" json:"error_message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type DeployLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeployID  uint      `gorm:"index;not null" json:"deploy_id"`
	Line      string    `gorm:"type:text;not null" json:"line"`
	IsError   bool      `gorm:"default:false" json:"is_error"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
