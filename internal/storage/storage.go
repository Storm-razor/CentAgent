package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Path            string           `mapstructure:"path"`
	InMemory        bool             `mapstructure:"in_memory"`
	EnableWAL       bool             `mapstructure:"enable_wal"`
	BusyTimeout     time.Duration    `mapstructure:"busy_timeout"`
	MaxOpenConns    int              `mapstructure:"max_open_conns"`
	MaxIdleConns    int              `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration    `mapstructure:"conn_max_lifetime"`
	Logger          logger.Interface `mapstructure:"-"`
}

type Storage struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func Open(ctx context.Context, cfg Config) (*Storage, error) {
	if cfg.BusyTimeout <= 0 {
		cfg.BusyTimeout = 5 * time.Second
	}

	dsn, err := dsnFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	gormCfg := &gorm.Config{}
	if cfg.Logger != nil {
		gormCfg.Logger = cfg.Logger
	}

	db, err := gorm.Open(sqlite.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	s := &Storage{db: db, sqlDB: sqlDB}

	if cfg.EnableWAL {
		if err := s.db.WithContext(ctx).Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("enable wal: %w", err)
		}
	}

	if err := s.db.WithContext(ctx).Exec("PRAGMA foreign_keys=ON;").Error; err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := s.Migrate(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}

	if err := s.Ping(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}

	return s, nil
}

func (s *Storage) Close() error {
	if s == nil || s.sqlDB == nil {
		return nil
	}
	return s.sqlDB.Close()
}

func (s *Storage) Ping(ctx context.Context) error {
	if s == nil || s.sqlDB == nil {
		return errors.New("storage not initialized")
	}
	return s.sqlDB.PingContext(ctx)
}

func (s *Storage) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialized")
	}

	// 临时修复：由于 AuditRecord 移除了 Actor 字段，但 SQLite 的 AutoMigrate 不会删除旧列/约束
	// 导致 NOT NULL constraint failed。这里检查如果表存在且有 actor 列，则重建表。
	if s.db.Migrator().HasTable(&AuditRecord{}) {
		if s.db.Migrator().HasColumn(&AuditRecord{}, "actor") {
			// 发现旧列，重建表
			if err := s.db.Migrator().DropTable(&AuditRecord{}); err != nil {
				return fmt.Errorf("drop old audit_records table: %w", err)
			}
		}
	}

	if err := s.db.WithContext(ctx).AutoMigrate(
		&ContainerStat{},
		&ContainerLog{},
		&AuditRecord{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

func (s *Storage) DB() *gorm.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func dsnFromConfig(cfg Config) (string, error) {
	timeoutMS := int(cfg.BusyTimeout / time.Millisecond)
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}

	if cfg.InMemory {
		return fmt.Sprintf("file:centagent?mode=memory&cache=shared&_busy_timeout=%d", timeoutMS), nil
	}

	if cfg.Path == "" {
		return "", errors.New("sqlite path is required when InMemory=false")
	}

	return fmt.Sprintf("file:%s?_busy_timeout=%d", cfg.Path, timeoutMS), nil
}
