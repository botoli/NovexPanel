package storage

import (
	"fmt"
	"log"

	"novexpanel/backend/internal/models"

	"gorm.io/gorm"
)

func runMigrations(db *gorm.DB) error {
	log.Printf("running database migrations")

	if err := db.AutoMigrate(
		&models.User{},
		&models.AgentToken{},
		&models.Server{},
		&models.MetricPoint{},
		&models.Deploy{},
		&models.DeployLog{},
	); err != nil {
		return fmt.Errorf("automigrate schema: %w", err)
	}

	if !db.Migrator().HasTable(&models.Deploy{}) {
		return fmt.Errorf("deploys table migration failed")
	}

	// Explicitly ensure subdirectory column exists for older databases.
	if !db.Migrator().HasColumn(&models.Deploy{}, "subdirectory") {
		if err := db.Migrator().AddColumn(&models.Deploy{}, "Subdirectory"); err != nil {
			return fmt.Errorf("add deploys.subdirectory column: %w", err)
		}
	}

	return nil
}
