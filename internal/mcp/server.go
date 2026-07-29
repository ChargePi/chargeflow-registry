package mcp

import (
	"context"

	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"github.com/ChargePi/chargeflow-registry/internal/validation"
	mcplogging_zap "github.com/ChargePi/chargex-sdk/mcp/logging/zap"
	mcp_tracing "github.com/ChargePi/chargex-sdk/mcp/tracing"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

// SchemaRegistry is the subset of schema.Service used by the MCP server.
type SchemaRegistry interface {
	Get(ctx context.Context, version schema.OCPPVersion, action string, msgType schema.MessageType, vendor, model *string) (*schema.Schema, error)
	AddPair(ctx context.Context, req, resp *schema.Schema) error
	Delete(ctx context.Context, version schema.OCPPVersion, action string, vendor, model *string) error
	List(ctx context.Context, version schema.OCPPVersion, vendor, model, action *string, msgType *schema.MessageType, status *schema.Status, limit, offset uint32) ([]*schema.Schema, int64, error)
}

// MessageValidator is the subset of validation.Service used by the MCP server.
type MessageValidator interface {
	ValidateMessage(ctx context.Context, req validation.Request) (*validation.Result, error)
}

type Server struct {
	http   *server.StreamableHTTPServer
	logger *zap.Logger
}

type handlers struct {
	schemaSvc     SchemaRegistry
	validationSvc MessageValidator
}

// NewServer creates an MCP server with schema tools and resources registered.
func NewServer(logger *zap.Logger, schemaSvc SchemaRegistry, validationSvc MessageValidator) *Server {
	h := &handlers{
		schemaSvc:     schemaSvc,
		validationSvc: validationSvc,
	}

	// Add logging and tracing hooks
	hooks := &server.Hooks{}
	mcplogging_zap.LoggingHooks(logger, hooks)
	mcp_tracing.AddHooks(hooks)

	mcp := server.NewMCPServer("chargeflow-registry", "1.0.0", server.WithHooks(hooks))

	registerTools(mcp, h)
	registerResources(mcp, h)

	s := server.NewStreamableHTTPServer(mcp)

	return &Server{http: s, logger: logger.Named("mcp-server")}
}

// Start runs the MCP Streamable HTTP server in a goroutine, logging fatal on failure.
func (s *Server) Start(addr string) {
	go func() {
		s.logger.Info("Starting MCP server", zap.String("address", addr))
		if err := s.http.Start(addr); err != nil {
			s.logger.Panic("MCP server error", zap.Error(err))
		}
	}()
}

// Shutdown gracefully stops the MCP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
