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
		&models.CommandLog{},
	); err != nil {
		return fmt.Errorf("automigrate schema: %w", err)
	}

	if !db.Migrator().HasTable(&models.Deploy{}) {
		return fmt.Errorf("deploys table migration failed")
	}

	if err := ensureDeployColumn(db, "subdirectory", "Subdirectory"); err != nil {
		return err
	}
	if err := ensureDeployColumn(db, "finished_at", "FinishedAt"); err != nil {
		return err
	}
	if err := ensureDeployColumn(db, "error_message", "ErrorMessage"); err != nil {
		return err
	}
	if err := ensureDeployColumn(db, "env_vars", "EnvVars"); err != nil {
		return err
	}

	return nil
}

func ensureDeployColumn(db *gorm.DB, dbColumn, modelField string) error {
	if db.Migrator().HasColumn(&models.Deploy{}, dbColumn) {
		return nil
	}
	if err := db.Migrator().AddColumn(&models.Deploy{}, modelField); err != nil {
		return fmt.Errorf("add deploys.%s column: %w", dbColumn, err)
	}
	return nil
}
