package tracer

import (
	"context"
	"reflect"
	"testing"

	"github.com/danbordeanu/go-logger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestInitTracerJaeger verifies that InitTracerJaeger behaves correctly in both positive and negative scenarios.
func TestInitTracerJaeger(t *testing.T) {
	type args struct {
		ctx                  context.Context
		JaegerEngine         string
		ServiceNameKey       string
		ServiceInstanceIDKey string
		tenant               string
	}

	ctx := context.Background()
	logger.Init(ctx, true, true)
	log := logger.SugaredLogger()
	//goland:noinspection GoUnhandledErrorResult
	defer log.Sync()
	defer logger.PanicLogger()

	log.Infof("Jaeger Telemetry enabled - Running Tests")

	tests := []struct {
		name    string
		args    args
		wantNil bool
		wantErr bool
	}{
		{
			name: "Valid input",
			args: args{
				ctx:                  context.Background(),
				JaegerEngine:         "localhost:4318",
				ServiceNameKey:       "MyService",
				ServiceInstanceIDKey: "Instance123",
				tenant:               "TenantABC",
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "Invalid JaegerEngine endpoint",
			args: args{
				ctx:                  context.Background(),
				JaegerEngine:         "http://invalid_endpoint:4318",
				ServiceNameKey:       "MyService",
				ServiceInstanceIDKey: "Instance123",
				tenant:               "TenantABC",
			},
			wantNil: true,
			wantErr: true,
		},
		{
			name: "Empty ServiceNameKey",
			args: args{
				ctx:                  context.Background(),
				JaegerEngine:         "localhost:4317",
				ServiceNameKey:       "",
				ServiceInstanceIDKey: "Instance123",
				tenant:               "TenantABC",
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "Empty ServiceInstanceIDKey",
			args: args{
				ctx:                  context.Background(),
				JaegerEngine:         "localhost:4317",
				ServiceNameKey:       "MyService",
				ServiceInstanceIDKey: "",
				tenant:               "TenantABC",
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "Empty tenant",
			args: args{
				ctx:                  context.Background(),
				JaegerEngine:         "localhost:4317",
				ServiceNameKey:       "MyService",
				ServiceInstanceIDKey: "Instance123",
				tenant:               "",
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InitTracerJaeger(tt.args.ctx, tt.args.JaegerEngine, tt.args.ServiceNameKey, tt.args.ServiceInstanceIDKey, tt.args.tenant)

			// Check if error matches expected
			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracerJaeger() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check if TracerProvider nil status matches expected
			if (got == nil) != tt.wantNil {
				t.Errorf("InitTracerJaeger() got = %v, wantNil %v", got, tt.wantNil)
			}

			// Optional: Add more checks on attributes in `got` if needed
		})
	}
}

// TestInitTracerStdout  verifies that InitTracerStdout behaves correctly
func TestInitTracerStdout(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	var tests []struct {
		name    string
		args    args
		want    *sdktrace.TracerProvider
		wantErr bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InitTracerStdout(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracerStdout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InitTracerStdout() got = %v, want %v", got, tt.want)
			}
		})
	}
}
