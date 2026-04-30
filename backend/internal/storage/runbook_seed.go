package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"novexpanel/backend/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type starterRunbookSeed struct {
	Title       string
	Slug        string
	Description string
	Tags        []string
	TargetType  string
	Definition  map[string]any
}

func seedStarterRunbooks(db *gorm.DB) error {
	var servers []models.Server
	if err := db.Find(&servers).Error; err != nil {
		return fmt.Errorf("load servers for runbook seed: %w", err)
	}

	for _, server := range servers {
		if err := seedStarterRunbooksForServer(db, server.UserID, server.ID); err != nil {
			return err
		}
	}

	return nil
}

func seedStarterRunbooksForServer(db *gorm.DB, userID, serverID uint) error {
	starter := defaultStarterRunbooks()
	for _, item := range starter {
		var existing models.Runbook
		err := db.Where("user_id = ? AND server_id = ? AND slug = ?", userID, serverID, item.Slug).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find starter runbook %s: %w", item.Slug, err)
		}

		raw, marshalErr := json.Marshal(item.Definition)
		if marshalErr != nil {
			return fmt.Errorf("marshal starter definition %s: %w", item.Slug, marshalErr)
		}

		runbook := models.Runbook{
			UserID:        userID,
			ServerID:      serverID,
			Title:         item.Title,
			Slug:          item.Slug,
			Description:   item.Description,
			Tags:          strings.Join(item.Tags, ","),
			TargetType:    item.TargetType,
			LatestVersion: 1,
		}
		if err := db.Create(&runbook).Error; err != nil {
			return fmt.Errorf("create starter runbook %s: %w", item.Slug, err)
		}

		version := models.RunbookVersion{
			RunbookID:      runbook.ID,
			Version:        1,
			DefinitionJSON: datatypes.JSON(raw),
			CreatedBy:      userID,
			ChangeNote:     "starter template",
		}
		if err := db.Create(&version).Error; err != nil {
			return fmt.Errorf("create starter runbook version %s: %w", item.Slug, err)
		}
	}
	return nil
}

func defaultStarterRunbooks() []starterRunbookSeed {
	return []starterRunbookSeed{
		{
			Title:       "Disk Full",
			Slug:        "disk-full",
			Description: "Check disk usage and clear temporary files.",
			Tags:        []string{"storage", "incident"},
			TargetType:  "server",
			Definition: map[string]any{
				"variables": []map[string]any{{"name": "cleanup_path", "default": "/tmp"}},
				"timeout":   300,
				"steps": []map[string]any{
					{"name": "inspect-disk", "type": "shell", "command": "df -h", "timeout": 20},
					{"name": "cleanup-temp", "type": "shell", "command": "du -sh {{cleanup_path}} && find {{cleanup_path}} -type f -mtime +3 -delete", "timeout": 120, "continue_on_error": true},
				},
			},
		},
		{
			Title:       "Nginx Down",
			Slug:        "nginx-down",
			Description: "Validate nginx service and restart it if needed.",
			Tags:        []string{"nginx", "availability"},
			TargetType:  "service",
			Definition: map[string]any{
				"variables": []map[string]any{{"name": "service_name", "default": "nginx"}},
				"timeout":   180,
				"steps": []map[string]any{
					{"name": "check-status", "type": "shell", "command": "systemctl status {{service_name}} --no-pager", "timeout": 30, "continue_on_error": true},
					{"name": "restart", "type": "shell", "command": "sudo systemctl restart {{service_name}}", "timeout": 30},
					{"name": "verify", "type": "shell", "command": "systemctl is-active {{service_name}}", "expected_output": "active", "timeout": 20},
				},
			},
		},
		{
			Title:       "Certificate Expired",
			Slug:        "cert-expired",
			Description: "Inspect certificate expiry and reload nginx.",
			Tags:        []string{"tls", "security"},
			TargetType:  "service",
			Definition: map[string]any{
				"variables": []map[string]any{{"name": "domain", "default": "example.com"}},
				"timeout":   300,
				"steps": []map[string]any{
					{"name": "check-expiry", "type": "shell", "command": "echo | openssl s_client -servername {{domain}} -connect {{domain}}:443 2>/dev/null | openssl x509 -noout -dates", "timeout": 45},
					{"name": "renew", "type": "shell", "command": "sudo certbot renew", "timeout": 180, "continue_on_error": true},
					{"name": "reload-nginx", "type": "shell", "command": "sudo systemctl reload nginx", "timeout": 30},
				},
			},
		},
		{
			Title:       "Restart Service",
			Slug:        "restart-service",
			Description: "Safe restart flow for any systemd service.",
			Tags:        []string{"service", "maintenance"},
			TargetType:  "service",
			Definition: map[string]any{
				"variables": []map[string]any{{"name": "service_name", "default": "app"}},
				"timeout":   180,
				"steps": []map[string]any{
					{"name": "pre-check", "type": "shell", "command": "systemctl status {{service_name}} --no-pager", "timeout": 30, "continue_on_error": true},
					{"name": "restart", "type": "shell", "command": "sudo systemctl restart {{service_name}}", "timeout": 30},
					{"name": "post-check", "type": "shell", "command": "systemctl is-active {{service_name}}", "expected_output": "active", "timeout": 20},
				},
			},
		},
		{
			Title:       "Rotate Logs",
			Slug:        "rotate-logs",
			Description: "Force logrotate and cleanup old logs.",
			Tags:        []string{"logs", "maintenance"},
			TargetType:  "server",
			Definition: map[string]any{
				"variables": []map[string]any{{"name": "log_path", "default": "/var/log"}},
				"timeout":   240,
				"steps": []map[string]any{
					{"name": "logrotate", "type": "shell", "command": "sudo logrotate -f /etc/logrotate.conf", "timeout": 40},
					{"name": "cleanup-old", "type": "shell", "command": "find {{log_path}} -type f -name '*.log.*' -mtime +14 -delete", "timeout": 60, "continue_on_error": true},
				},
			},
		},
	}
}
