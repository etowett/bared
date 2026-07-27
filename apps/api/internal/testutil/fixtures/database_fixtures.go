// Package fixtures provides test fixtures for database configurations and mock data.
package fixtures

import (
	"github.com/etowett/bared/apps/api/internal/config"
)

// MySQLConnection returns a valid MySQL connection config for testing
func MySQLConnection() *config.Connection {
	return &config.Connection{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "testpass",
		Database: "testdb",
	}
}

// PostgresConnection returns a valid PostgreSQL connection config for testing
func PostgresConnection() *config.Connection {
	return &config.Connection{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "testpass",
		Database: "testdb",
	}
}

// RedisConnection returns a valid Redis connection config for testing
func RedisConnection() *config.Connection {
	return &config.Connection{
		Type:     "redis",
		Host:     "localhost",
		Port:     6379,
		Password: "testpass",
	}
}

// MySQLTarget returns a complete MySQL target config for testing
func MySQLTarget() *config.Target {
	return &config.Target{
		Name: "mysql_test",
		Conn: MySQLConnection(),
		Compress: &config.CompressionOpts{
			Enabled: true,
			Type:    "tgz",
		},
		Storage: &config.TargetStorage{
			Enabled: true,
			Name:    "local",
		},
	}
}

// PostgresTarget returns a complete PostgreSQL target config for testing
func PostgresTarget() *config.Target {
	return &config.Target{
		Name: "postgres_test",
		Conn: PostgresConnection(),
		Compress: &config.CompressionOpts{
			Enabled: true,
			Type:    "tgz",
		},
		Storage: &config.TargetStorage{
			Enabled: true,
			Name:    "local",
		},
	}
}

// RedisTarget returns a complete Redis target config for testing
func RedisTarget() *config.Target {
	return &config.Target{
		Name: "redis_test",
		Conn: RedisConnection(),
		Compress: &config.CompressionOpts{
			Enabled: true,
			Type:    "tgz",
		},
		Storage: &config.TargetStorage{
			Enabled: true,
			Name:    "local",
		},
	}
}

// MySQLTargetWithExcludeTables returns MySQL target with table exclusions
func MySQLTargetWithExcludeTables() *config.Target {
	target := MySQLTarget()
	target.ExcludeTables = []string{"temp_table", "cache_table"}
	return target
}

// MySQLTargetWithAdditionalArgs returns MySQL target with additional args
func MySQLTargetWithAdditionalArgs() *config.Target {
	target := MySQLTarget()
	target.AdditionalArgs = []string{"--single-transaction", "--quick"}
	return target
}

// PostgresTargetWithAdditionalArgs returns Postgres target with additional args
func PostgresTargetWithAdditionalArgs() *config.Target {
	target := PostgresTarget()
	target.AdditionalArgs = []string{"--format=custom", "--compress=9"}
	return target
}

// MockDumpData returns sample database dump data for testing
func MockDumpData() string {
	return `-- MySQL dump 10.13
--
-- Host: localhost    Database: testdb
--
-- Table structure for table users
--
CREATE TABLE users (
  id int NOT NULL AUTO_INCREMENT,
  name varchar(255) DEFAULT NULL,
  email varchar(255) DEFAULT NULL,
  PRIMARY KEY (id)
);

--
-- Dumping data for table users
--
INSERT INTO users VALUES (1,'Test User','test@example.com');
INSERT INTO users VALUES (2,'Another User','another@example.com');
`
}

// MockPostgresDumpData returns sample PostgreSQL dump data for testing
func MockPostgresDumpData() string {
	return `--
-- PostgreSQL database dump
--
-- Dumped from database version 14.5
-- Dumped by pg_dump version 14.5

SET statement_timeout = 0;
SET lock_timeout = 0;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id integer NOT NULL,
    name character varying(255),
    email character varying(255)
);

--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.users (id, name, email) FROM stdin;
1	Test User	test@example.com
2	Another User	another@example.com
\.
`
}

// MockRedisDumpData returns sample Redis RDB dump data (binary format simulation)
func MockRedisDumpData() string {
	// This is a simplified text representation of Redis data
	// In reality, RDB files are binary
	return `REDIS0009
SET mykey myvalue
SET another_key another_value
HSET user:1 name "Test User"
HSET user:1 email "test@example.com"
`
}

// LargeMockDumpData returns a larger dump for testing streaming
func LargeMockDumpData(sizeKB int) string {
	line := "INSERT INTO test_table VALUES (1, 'test data with some content');\n"
	lineSize := len(line)
	iterations := (sizeKB * 1024) / lineSize

	result := "-- Large MySQL dump for testing\n"
	for i := 0; i < iterations; i++ {
		result += line
	}
	return result
}
