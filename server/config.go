// Copyright 2017 The Nakama Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/satori/go.uuid"
)

// Config interface is the Nakama Core configuration
type Config interface {
	GetName() string
	GetDataDir() string
	GetPort() int
	GetOpsPort() int
	GetDSNS() []string
	GetSession() *SessionConfig
	GetTransport() *TransportConfig
	GetDatabase() *DatabaseConfig
	GetSocial() *SocialConfig
}

type config struct {
	Name      string           `yaml:"name" json:"name" flag:"name"`
	Datadir   string           `yaml:"data_dir" json:"data_dir" flag:"data-dir"`
	Port      int              `yaml:"port" json:"port" flag:"port"`
	OpsPort   int              `yaml:"ops_port" json:"ops_port" flag:"ops-port"`
	Dsns      []string         `yaml:"dsns" json:"dsns" flag:"dsns"`
	Session   *SessionConfig   `yaml:"session" json:"session"  flag:"session"`
	Transport *TransportConfig `yaml:"transport" json:"transport" flag:"transport"`
	Database  *DatabaseConfig  `yaml:"database" json:"database" flag:"database"`
	Social    *SocialConfig    `yaml:"social" json:"social" flag:"social"`
}

// NewConfig constructs a Config struct which represents server settings.
func NewConfig() *config {
	cwd, _ := os.Getwd()
	dataDirectory := filepath.FromSlash(cwd + "/data")
	nodeName := "nakama-" + strings.Split(uuid.NewV4().String(), "-")[3]
	return &config{
		Name:      nodeName,
		Datadir:   dataDirectory,
		Port:      7350,
		OpsPort:   7351,
		Session:   NewSessionConfig(),
		Transport: NewTransportConfig(),
		Database:  NewDatabaseConfig(),
		Social:    NewSocialConfig(),
	}
}

func (c *config) GetName() string {
	return c.Name
}

func (c *config) GetDataDir() string {
	return c.Datadir
}

func (c *config) GetPort() int {
	return c.Port
}

func (c *config) GetOpsPort() int {
	return c.OpsPort
}

func (c *config) GetDSNS() []string {
	return c.Dsns
}

func (c *config) GetSession() *SessionConfig {
	return c.Session
}

func (c *config) GetTransport() *TransportConfig {
	return c.Transport
}

func (c *config) GetDatabase() *DatabaseConfig {
	return c.Database
}

func (c *config) GetSocial() *SocialConfig {
	return c.Social
}

// SessionConfig is configuration relevant to the session
type SessionConfig struct {
	EncryptionKey string `yaml:"encryption_key" json:"encryption_key" flag:"encryption-key"`
	TokenExpiryMs int64  `yaml:"token_expiry_ms" json:"token_expiry_ms" flag:"token-expiry-ms"`
}

// NewSessionConfig creates a new SessionConfig struct
func NewSessionConfig() *SessionConfig {
	return &SessionConfig{
		EncryptionKey: "defaultencryptionkey",
		TokenExpiryMs: 60000,
	}
}

// TransportConfig is configuration relevant to the transport socket and protocol
type TransportConfig struct {
	ServerKey           string `yaml:"server_key" json:"server_key" flag:"server-key"`
	MaxMessageSizeBytes int64  `yaml:"max_message_size_bytes" json:"max_message_size_bytes" flag:"max-message-size-bytes"`
	WriteWaitMs         int    `yaml:"write_wait_ms" json:"write_wait_ms" flag:"write-wait-ms"`
	PongWaitMs          int    `yaml:"pong_wait_ms" json:"pong_wait_ms" flag:"pong-wait-ms"`
	PingPeriodMs        int    `yaml:"ping_period_ms" json:"ping_period_ms" flag:"ping-period-ms"`
}

// NewTransportConfig creates a new TransportConfig struct
func NewTransportConfig() *TransportConfig {
	return &TransportConfig{
		ServerKey:           "defaultkey",
		MaxMessageSizeBytes: 1024,
		WriteWaitMs:         5000,
		PongWaitMs:          10000,
		PingPeriodMs:        8000,
	}
}

// DatabaseConfig is configuration relevant to the Database storage
type DatabaseConfig struct {
	ConnMaxLifetimeMs int `yaml:"conn_max_lifetime_ms" json:"conn_max_lifetime_ms" flag:"conn-max-lifetime-ms"`
	MaxOpenConns      int `yaml:"max_open_conns" json:"max_open_conns" flag:"max-open-conns"`
	MaxIdleConns      int `yaml:"max_idle_conns" json:"max_idle_conns" flag:"max-idle-conns"`
}

// NewDatabaseConfig creates a new DatabaseConfig struct
func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		ConnMaxLifetimeMs: 60000,
		MaxOpenConns:      0,
		MaxIdleConns:      0,
	}
}

// SocialConfig is configuration relevant to the Social providers
type SocialConfig struct {
	Steam *SocialConfigSteam `yaml:"steam" json:"steam" flag:"steam"`
}

// SocialConfigSteam is configuration relevant to Steam
type SocialConfigSteam struct {
	PublisherKey string `yaml:"publisher_key" json:"publisher_key" flag:"publisher-key"`
	AppID        int    `yaml:"app_id" json:"app_id" flag:"app-id"`
}

// NewSocialConfig creates a new SocialConfig struct
func NewSocialConfig() *SocialConfig {
	return &SocialConfig{
		Steam: &SocialConfigSteam{
			PublisherKey: "",
			AppID:        0,
		},
	}
}
