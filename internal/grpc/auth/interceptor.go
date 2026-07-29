package auth

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Role represents a user role controlling access to gRPC methods.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// roleLevel assigns a numeric level so higher roles satisfy lower requirements.
var roleLevel = map[Role]int{
	RoleReader: 0,
	RoleWriter: 1,
	RoleAdmin:  2,
}

// Claims is the JWT payload expected in every request token.
type Claims struct {
	jwt.RegisteredClaims
	Roles []Role `json:"roles"`
}

type contextKey struct{}

// ClaimsFromContext retrieves the validated JWT claims stored by the interceptor.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(*Claims)
	return c, ok
}

// methodRoles maps each full gRPC method path to the minimum role required.
// Methods not listed here (health, reflection) are allowed through unauthenticated.
var methodRoles = map[string]Role{
	"/schema.v1.SchemaRegistryService/AddSchema":         RoleWriter,
	"/schema.v1.SchemaRegistryService/UpsertSchema":      RoleWriter,
	"/schema.v1.SchemaRegistryService/DeleteSchema":      RoleWriter,
	"/schema.v1.SchemaValidationService/ValidateMessage": RoleReader,
	"/admin.v1.AdminService/ListSchemas":                 RoleAdmin,
	"/admin.v1.AdminService/ChangeStatus":                RoleAdmin,
}

// Interceptor validates JWT bearer tokens and enforces role-based access on gRPC methods.
type Interceptor struct {
	secret []byte
}

func NewInterceptor(secret []byte) *Interceptor {
	return &Interceptor{secret: secret}
}

func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := i.authorize(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

func (i *Interceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		_, err := i.authorize(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func (i *Interceptor) authorize(ctx context.Context, method string) (context.Context, error) {
	required, exists := methodRoles[method]
	if !exists {
		// health checks, reflection, etc. — no auth required
		return ctx, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return ctx, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	raw, found := strings.CutPrefix(values[0], "Bearer ")
	if !found {
		return ctx, status.Error(codes.Unauthenticated, "authorization header must use Bearer scheme")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, status.Errorf(codes.Unauthenticated, "unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil || !token.Valid {
		return ctx, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	if !hasRequiredRole(claims.Roles, required) {
		return ctx, status.Errorf(codes.PermissionDenied, "role %q or higher required", required)
	}

	return context.WithValue(ctx, contextKey{}, claims), nil
}

// hasRequiredRole returns true if any of the user's roles meets or exceeds the required level.
func hasRequiredRole(userRoles []Role, required Role) bool {
	threshold := roleLevel[required]
	for _, r := range userRoles {
		if level, ok := roleLevel[r]; ok && level >= threshold {
			return true
		}
	}
	return false
}
