package models

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Email        string `gorm:"uniqueIndex;size:190;not null" json:"email"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Role         string `gorm:"size:32;not null;default:''" json:"role"`
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
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"index;not null" json:"user_id"`
	ServerID     uint           `gorm:"index;not null" json:"server_id"`
	Branch       string         `gorm:"size:120;default:main" json:"branch"`
	BuildCommand string         `gorm:"size:512" json:"build_command"`
	OutputDir    string         `gorm:"size:512" json:"output_dir"`
	EnvVars      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"env_vars"`
	Subdirectory string         `gorm:"type:varchar(255);default:''" json:"subdirectory"`
	Port         int            `gorm:"default:0" json:"port"`
	Source       string         `gorm:"size:20;not null" json:"source"`
	Status       string         `gorm:"size:20;index;not null" json:"status"`
	ProjectType  string         `gorm:"size:20" json:"project_type"`
	RepoURL      string         `gorm:"size:512" json:"repo_url"`
	CommitHash   string         `gorm:"size:64" json:"commit_hash"`
	CommitAuthor string         `gorm:"size:255" json:"commit_author"`
	CommitMsg    string         `gorm:"size:512" json:"commit_message"`
	URL          string         `gorm:"size:512" json:"url"`
	DeployLog    string         `gorm:"type:text" json:"deploy_log"`
	ResultURL    string         `gorm:"size:512" json:"result_url"`
	ErrorMessage string         `gorm:"size:1024" json:"error_message"`
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type DeployLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeployID  uint      `gorm:"index;not null" json:"deploy_id"`
	Line      string    `gorm:"type:text;not null" json:"line"`
	IsError   bool      `gorm:"default:false" json:"is_error"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// CommandLog records remote shell commands issued via POST /servers/:id/command (audit).
type CommandLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ServerID  uint      `gorm:"index;not null" json:"server_id"`
	Command   string    `gorm:"size:255;not null" json:"command"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type GitHubConnection struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"uniqueIndex;not null" json:"user_id"`
	GitHubUserID    int64      `gorm:"index;not null" json:"github_user_id"`
	Login           string     `gorm:"size:120;not null" json:"login"`
	AvatarURL       string     `gorm:"size:512" json:"avatar_url"`
	AccessTokenEnc  string     `gorm:"type:text;not null" json:"-"`
	Scope           string     `gorm:"size:512" json:"scope"`
	ConnectedAt     time.Time  `json:"connected_at"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	TokenUpdatedAt  time.Time  `json:"token_updated_at"`
	InstallationIDs string     `gorm:"size:512" json:"installation_ids"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ProjectAccess struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Email       string    `gorm:"size:190;not null" json:"email"`
	Role        string    `gorm:"size:32;index;not null" json:"role"`
	InvitedByID uint      `gorm:"index;not null" json:"invited_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type APIToken struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	Name        string     `gorm:"size:120;not null" json:"name"`
	TokenHash   string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	TokenPrefix string     `gorm:"size:24;not null" json:"token_prefix"`
	Revoked     bool       `gorm:"index;default:false" json:"revoked"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Runbook struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"index;not null" json:"user_id"`
	ServerID      uint       `gorm:"index:idx_runbook_server_slug,unique;not null" json:"server_id"`
	Title         string     `gorm:"size:180;not null" json:"title"`
	Slug          string     `gorm:"size:120;not null;index:idx_runbook_server_slug,unique" json:"slug"`
	Description   string     `gorm:"type:text" json:"description"`
	Tags          string     `gorm:"size:512" json:"tags"`
	TargetType    string     `gorm:"size:32;not null" json:"target_type"`
	LatestVersion int        `gorm:"not null;default:1" json:"latest_version"`
	LastRunAt     *time.Time `json:"last_run_at"`
	LastRunStatus string     `gorm:"size:24" json:"last_run_status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type RunbookVersion struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	RunbookID       uint           `gorm:"index;not null" json:"runbook_id"`
	Version         int            `gorm:"not null" json:"version"`
	DefinitionJSON  datatypes.JSON `gorm:"type:jsonb;not null" json:"definition_json"`
	CreatedBy       uint           `gorm:"index;not null" json:"created_by"`
	ChangeNote      string         `gorm:"size:255" json:"change_note"`
	IsRollbackPoint bool           `gorm:"default:false" json:"is_rollback_point"`
	CreatedAt       time.Time      `json:"created_at"`
}

type RunbookExecution struct {
	ID                        uint       `gorm:"primaryKey" json:"id"`
	RunbookID                 uint       `gorm:"index;not null" json:"runbook_id"`
	RunbookVersionID          uint       `gorm:"index;not null" json:"runbook_version_id"`
	DryRun                    bool       `gorm:"default:false" json:"dry_run"`
	Status                    string     `gorm:"size:24;index;not null" json:"status"`
	StartedAt                 *time.Time `json:"started_at"`
	FinishedAt                *time.Time `json:"finished_at"`
	TriggeredBy               uint       `gorm:"index;not null" json:"triggered_by"`
	RolledBackFromExecutionID *uint      `gorm:"index" json:"rolled_back_from_execution_id"`
	Summary                   string     `gorm:"type:text" json:"summary"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type RunbookExecutionLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ExecutionID uint      `gorm:"index;not null" json:"execution_id"`
	StepIndex   int       `gorm:"index;not null;default:0" json:"step_index"`
	StepName    string    `gorm:"size:180" json:"step_name"`
	Status      string    `gorm:"size:24;index;not null" json:"status"`
	Line        string    `gorm:"type:text;not null" json:"line"`
	Stream      string    `gorm:"size:16;not null" json:"stream"`
	Attempt     int       `gorm:"not null;default:1" json:"attempt"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

type RunbookAuditEvent struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index;not null" json:"user_id"`
	ServerID    uint           `gorm:"index;not null" json:"server_id"`
	RunbookID   *uint          `gorm:"index" json:"runbook_id"`
	ExecutionID *uint          `gorm:"index" json:"execution_id"`
	Action      string         `gorm:"size:80;index;not null" json:"action"`
	PayloadJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"payload_json"`
	CreatedAt   time.Time      `gorm:"index" json:"created_at"`
}

type ServiceActionLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	ServerID     uint      `gorm:"index;not null" json:"server_id"`
	Provider     string    `gorm:"size:32;index;not null" json:"provider"`
	ServiceName  string    `gorm:"size:180;index;not null" json:"service_name"`
	Action       string    `gorm:"size:32;index;not null" json:"action"`
	Graceful     bool      `gorm:"default:false" json:"graceful"`
	Success      bool      `gorm:"default:false" json:"success"`
	ErrorMessage string    `gorm:"size:512" json:"error_message"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

type FileOpVersion struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index;not null" json:"user_id"`
	ServerID   uint           `gorm:"index;not null" json:"server_id"`
	Path       string         `gorm:"size:512;index;not null" json:"path"`
	Checksum   string         `gorm:"size:128;index;not null" json:"checksum"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	Provider   string         `gorm:"size:32;index;not null" json:"provider"`
	Validation datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"validation"`
	CreatedBy  uint           `gorm:"index;not null" json:"created_by"`
	CreatedAt  time.Time      `gorm:"index" json:"created_at"`
}

type FileOpAuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ServerID  uint      `gorm:"index;not null" json:"server_id"`
	Path      string    `gorm:"size:512;index;not null" json:"path"`
	Provider  string    `gorm:"size:32;index;not null" json:"provider"`
	Action    string    `gorm:"size:32;index;not null" json:"action"`
	Success   bool      `gorm:"default:false" json:"success"`
	Message   string    `gorm:"size:1024" json:"message"`
	Checksum  string    `gorm:"size:128;index" json:"checksum"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type SecretVaultItem struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	ServerID       uint       `gorm:"index;not null" json:"server_id"`
	ServiceScope   string     `gorm:"size:180;index" json:"service_scope"`
	Name           string     `gorm:"size:180;index;not null" json:"name"`
	Type           string     `gorm:"size:32;index;not null" json:"type"`
	EncryptedValue string     `gorm:"type:text;not null" json:"-"`
	EncryptedDEK   string     `gorm:"type:text;not null" json:"-"`
	Nonce          string     `gorm:"size:64;not null" json:"-"`
	KeyVersion     int        `gorm:"not null;default:1" json:"key_version"`
	MaskedValue    string     `gorm:"size:255;not null" json:"masked_value"`
	LastRotatedAt  *time.Time `json:"last_rotated_at"`
	ExpiresAt      *time.Time `gorm:"index" json:"expires_at"`
	RevokedAt      *time.Time `gorm:"index" json:"revoked_at"`
	UsageCount     int        `gorm:"not null;default:0" json:"usage_count"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type SecretVaultAuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ServerID  uint      `gorm:"index;not null" json:"server_id"`
	SecretID  *uint     `gorm:"index" json:"secret_id"`
	Action    string    `gorm:"size:40;index;not null" json:"action"`
	Target    string    `gorm:"size:64;index" json:"target"`
	Message   string    `gorm:"size:1024" json:"message"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
