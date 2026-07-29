package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	grpcHandler "github.com/ChargePi/chargeflow-registry/internal/grpc"
	"github.com/ChargePi/chargeflow-registry/internal/grpc/adminserver"
	"github.com/ChargePi/chargeflow-registry/internal/grpc/auth"
	mcpHandler "github.com/ChargePi/chargeflow-registry/internal/mcp"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	postgresStorage "github.com/ChargePi/chargeflow-registry/internal/storage/postgres"
	redisStorage "github.com/ChargePi/chargeflow-registry/internal/storage/redis"
	"github.com/ChargePi/chargeflow-registry/internal/validation"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/redis/go-redis/extra/redisotel-native/v9"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	devxCfg "github.com/xBlaz3kx/DevX/configuration"
	"moul.io/zapgorm2"
	"github.com/xBlaz3kx/DevX/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	serviceName    = "chargeflow-registry"
	serviceVersion = "1.0.0-beta"
)

var (
	configurationFile string

	rootCmd = &cobra.Command{
		Use:     "chargeflow-registry",
		Short:   "an OCPP schema compatibility registry.",
		Long:    `chargeflow-registry is an OCPP schema compatibility registry.`,
		Version: serviceVersion,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg := getConfiguration()

			obs, err := observability.NewObservability(ctx, observability.ServiceInfo{
				Name:    serviceName,
				Version: serviceVersion,
			}, cfg.Observability)
			if err != nil {
				zap.L().Fatal("failed to setup observability", zap.Error(err))
			}
			defer obs.Shutdown(ctx)

			logger := zap.L()

			gormLogger := zapgorm2.New(logger)
			gormLogger.SetAsDefault()

			db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{Logger: gormLogger})
			if err != nil {
				logger.Fatal("failed to connect to database", zap.Error(err))
			}
			if err := db.Use(otelgorm.NewPlugin()); err != nil {
				logger.Fatal("failed to setup postgres tracing", zap.Error(err))
			}

			redisObs := redisotel.GetObservabilityInstance()
			if err := redisObs.Init(redisotel.NewConfig().WithEnabled(true)); err != nil {
				logger.Fatal("failed to setup redis observability", zap.Error(err))
			}
			defer func() {
				if err := redisObs.Shutdown(); err != nil {
					logger.Error("failed to shut down redis observability", zap.Error(err))
				}
			}()

			redisClient := redis.NewClient(&redis.Options{
				Addr:     cfg.Redis.Address,
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			})

			repo := postgresStorage.NewRepository(db)
			cache := redisStorage.NewCache(redisClient, cfg.Redis.CacheTTL)

			schemaSvc := schema.NewService(repo, cache)
			validationSvc := validation.NewService(schemaSvc, logger)

			authInterceptor := auth.NewInterceptor([]byte(cfg.Auth.JWTSecret))

			recoveryHandler := func(p any) error {
				logger.Error("recovered from panic", zap.Any("panic", p), zap.String("stack", string(debug.Stack())))
				return status.Errorf(codes.Internal, "%s", p)
			}

			grpcServer := grpc.NewServer(
				grpc.StatsHandler(otelgrpc.NewServerHandler()),
				grpc.ChainUnaryInterceptor(
					grpc_zap.UnaryServerInterceptor(logger),
					grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler)),
					authInterceptor.Unary(),
				),
				grpc.ChainStreamInterceptor(
					grpc_zap.StreamServerInterceptor(logger),
					grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler)),
					authInterceptor.Stream(),
				),
			)

			grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
			schemav1.RegisterSchemaRegistryServiceServer(grpcServer, grpcHandler.NewHandler(schemaSvc))
			schemav1.RegisterSchemaValidationServiceServer(grpcServer, grpcHandler.NewValidationHandler(validationSvc))

			lis, err := net.Listen("tcp", cfg.GRPC.Address)
			if err != nil {
				logger.Fatal("failed to listen", zap.Error(err))
			}

			go func() {
				logger.Info("Starting gRPC server", zap.String("address", cfg.GRPC.Address))
				if err := grpcServer.Serve(lis); err != nil {
					logger.Fatal("failed to serve gRPC", zap.Error(err))
				}
			}()
			defer grpcServer.GracefulStop()

			// AdminAPI is served on its own gRPC server/port so it can be network-isolated
			// from the general registry API independently of role-based auth.
			adminGrpcServer := adminserver.NewServer(logger, authInterceptor, schemaSvc)
			if err := adminGrpcServer.Start(cfg.AdminGRPC.Address); err != nil {
				logger.Fatal("failed to start admin gRPC server", zap.Error(err))
			}
			defer adminGrpcServer.Shutdown(ctx)

			mcpServer := mcpHandler.NewServer(logger.Named("mcp"), schemaSvc, validationSvc)
			mcpServer.Start(cfg.MCP.Address)
			defer func() {
				if err := mcpServer.Shutdown(ctx); err != nil {
					logger.Error("MCP server shutdown error", zap.Error(err))
				}
			}()

			<-ctx.Done()
			logger.Info("Shutting down")
		},
	}
)

func InitConfig(configurationFilePath string) {
	setDefaults()
	devxCfg.SetupEnv("chargeflow_registry")
	devxCfg.InitConfig(configurationFilePath, "$HOME/chargeflow-registry/", "/usr/chargeflow-registry/config/")
}

func setDefaults() {
	devxCfg.SetDefaults(serviceName)
	viper.SetDefault("grpc.address", "0.0.0.0:50051")
	viper.SetDefault("adminGrpc.address", "0.0.0.0:50052")
	viper.SetDefault("mcp.address", "0.0.0.0:8080")
	viper.SetDefault("redis.address", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.cacheTtl", time.Hour)
	_ = viper.BindEnv("auth.jwtSecret", "CHARGEFLOW_REGISTRY_AUTH_JWTSECRET")
	_ = viper.BindEnv("database.dsn", "CHARGEFLOW_REGISTRY_DATABASE_DSN")
	_ = viper.BindEnv("redis.address", "CHARGEFLOW_REGISTRY_REDIS_ADDRESS")
	_ = viper.BindEnv("redis.password", "CHARGEFLOW_REGISTRY_REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "CHARGEFLOW_REGISTRY_REDIS_DB")
	_ = viper.BindEnv("redis.cacheTtl", "CHARGEFLOW_REGISTRY_REDIS_CACHETTL")
	_ = viper.BindEnv("grpc.address", "CHARGEFLOW_REGISTRY_GRPC_ADDRESS")
	_ = viper.BindEnv("adminGrpc.address", "CHARGEFLOW_REGISTRY_ADMIN_GRPC_ADDRESS")
	_ = viper.BindEnv("mcp.address", "CHARGEFLOW_REGISTRY_MCP_ADDRESS")
}

func getConfiguration() *Configuration {
	logger := zap.L()
	logger.Info("Getting configuration")
	defer logger.Info("Loaded and validated configuration!")

	var config Configuration
	devxCfg.GetConfiguration(viper.GetViper(), &config)
	return &config
}

func setupGlobalLogger() {
	logger, _ := zap.NewProduction()
	zap.ReplaceGlobals(logger)
}

func initConfig() {
	InitConfig(configurationFile)
}

func main() {
	rootCmd.PersistentFlags().StringVar(&configurationFile, "config", "", "configuration file path")

	cobra.OnInitialize(setupGlobalLogger, initConfig)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		zap.L().Fatal("Unable to run", zap.Error(err))
	}
}
