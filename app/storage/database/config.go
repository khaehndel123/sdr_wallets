package database

import (
	"fmt"
)

type Config struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	MigrationsTable string `mapstructure:"migrationsTable"`
}

func (c *Config) DBConnectionString() string {
	// dbname = user by convention
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		c.Host, c.Port, c.User, c.User, c.Password,
	)
}

func (c *Config) DBConnectionStringForMigration() string {
	// dbname = user by convention
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=disable&x-migrations-table=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.User,
		c.MigrationsTable,
	)
}
