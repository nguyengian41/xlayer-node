package db

import (
	"context"
	"fmt"

	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/gobuffalo/packr/v2"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgx/v4/stdlib"
	migrate "github.com/rubenv/sql-migrate"
)

const (
	// StateMigrationName is the name of the migration used by packr to pack the migration file
	StateMigrationName = "zkevm-state-db"
	// PoolMigrationName is the name of the migration used by packr to pack the migration file
	PoolMigrationName = "zkevm-pool-db"
	maxPlanMigration  = 1000
)

var packrMigrations = map[string]*packr.Box{
	StateMigrationName: packr.New(StateMigrationName, "./migrations/state"),
	PoolMigrationName:  packr.New(PoolMigrationName, "./migrations/pool"),
}

// NewSQLDB creates a new SQL DB
func NewSQLDB(cfg Config) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?pool_max_conns=%d", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.MaxConns))
	if err != nil {
		log.Errorf("Unable to parse DB config: %v\n", err)
		return nil, err
	}
	if cfg.EnableLog {
		config.ConnConfig.Logger = logger{}
	}
	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Errorf("Unable to connect to database: %v\n", err)
		return nil, err
	}
	return conn, nil
}

// RunMigrationsUp runs migrate-up for the given config.
func RunMigrationsUp(cfg Config, name string) error {
	return runMigrations(cfg, name, migrate.Up)
}

// CheckMigrations runs migrate-up for the given config.
func CheckMigrations(cfg Config, name string) error {
	return checkMigrations(cfg, name, migrate.Up)
}

// RunMigrationsDown runs migrate-down for the given config.
func RunMigrationsDown(cfg Config, name string) error {
	return runMigrations(cfg, name, migrate.Down)
}

// runMigrations will execute pending migrations if needed to keep
// the database updated with the latest changes in either direction,
// up or down.
func runMigrations(cfg Config, packrName string, direction migrate.MigrationDirection) error {
	c, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name))
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*c)

	box, ok := packrMigrations[packrName]
	if !ok {
		return fmt.Errorf("packr box not found with name: %v", packrName)
	}

	var migrations = &migrate.PackrMigrationSource{Box: box}
	nMigrations, err := migrate.Exec(db, "postgres", migrations, direction)
	if err != nil {
		return err
	}

	log.Info("successfully ran ", nMigrations, " migrations")
	return nil
}

func checkMigrations(cfg Config, packrName string, direction migrate.MigrationDirection) error {
	c, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name))
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*c)

	box, ok := packrMigrations[packrName]
	if !ok {
		return fmt.Errorf("packr box not found with name: %v", packrName)
	}

	var migrations = &migrate.PackrMigrationSource{Box: box}
	planMigrations, _, err := migrate.PlanMigration(db, "postgres", migrations, direction, maxPlanMigration)
	if err != nil {
		log.Error("error planning migrations. Error: ", err)
		return err
	}
	nmigrations := len(planMigrations)
	if nmigrations != 0 {
		log.Errorf("error the component needs to run %d migrations before starting", nmigrations)
		records, err := migrate.GetMigrationRecords(db, "postgres")
		if err != nil {
			log.Error("error getting migration records. Error: ", err)
			return err
		}
		return fmt.Errorf("error the component needs to run %d migrations before starting. DB only contains %d migrations", nmigrations, len(records))
	}
	return nil
}
