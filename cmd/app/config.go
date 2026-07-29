package main

import (
	"time"

	"github.com/xBlaz3kx/DevX/observability"
)

type DatabaseConfiguration struct {
	DSN string `json:"dsn" yaml:"dsn" mapstructure:"dsn" validate:"required"`
}

type RedisConfiguration struct {
	Address  string        `json:"address"  yaml:"address"  mapstructure:"address"  validate:"required"`
	Password string        `json:"password" yaml:"password" mapstructure:"password"`
	DB       int           `json:"db"       yaml:"db"       mapstructure:"db"`
	CacheTTL time.Duration `json:"cacheTtl" yaml:"cacheTtl" mapstructure:"cacheTtl"`
}

type AuthConfiguration struct {
	JWTSecret string `json:"jwtSecret" yaml:"jwtSecret" mapstructure:"jwtSecret" validate:"required"`
}

type GRPCConfiguration struct {
	Address string `json:"address" yaml:"address" mapstructure:"address" validate:"required"`
}

// AdminGRPCConfiguration configures the separate gRPC server exposing the AdminAPI.
type AdminGRPCConfiguration struct {
	Address string `json:"address" yaml:"address" mapstructure:"address" validate:"required"`
}

type MCPConfiguration struct {
	Address string `json:"address" yaml:"address" mapstructure:"address" validate:"required"`
}

type Configuration struct {
	Database      DatabaseConfiguration  `json:"database"      yaml:"database"      mapstructure:"database"      validate:"required"`
	Redis         RedisConfiguration     `json:"redis"         yaml:"redis"         mapstructure:"redis"         validate:"required"`
	Observability observability.Config   `json:"observability" yaml:"observability" mapstructure:"observability" validate:"required"`
	GRPC          GRPCConfiguration      `json:"grpc"          yaml:"grpc"          mapstructure:"grpc"          validate:"required"`
	AdminGRPC     AdminGRPCConfiguration `json:"adminGrpc"     yaml:"adminGrpc"     mapstructure:"adminGrpc"     validate:"required"`
	Auth          AuthConfiguration      `json:"auth"          yaml:"auth"          mapstructure:"auth"          validate:"required"`
	MCP           MCPConfiguration       `json:"mcp"           yaml:"mcp"           mapstructure:"mcp"           validate:"required"`
}
