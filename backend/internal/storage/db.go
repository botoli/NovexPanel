package storage

import (
	"fmt"
	"strings"

	"novexpanel/backend/internal/config"
	"novexpanel/backend/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	if strings.HasPrefix(cfg.DatabaseURL, "postgres://") || strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		dialector = postgres.Open(cfg.DatabaseURL)
	} else {
		dialector = sqlite.Open(cfg.DatabaseURL)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)

	if err := db.AutoMigrate(
		&models.User{},
		&models.AgentToken{},
		&models.Server{},
		&models.MetricPoint{},
		&models.Deploy{},
		&models.DeployLog{},
	); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}

	return db, nil
}
