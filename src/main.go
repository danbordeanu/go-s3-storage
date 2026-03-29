package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"s3-storage/api"
	"s3-storage/configuration"
	"s3-storage/docs"
	"s3-storage/services"
	"syscall"
	"time"

	"s3-storage/tracer"

	"github.com/danbordeanu/go-logger"
	"github.com/danbordeanu/go-stats/concurrency"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/automaxprocs/maxprocs"
)

// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @query.collection.format multi
func main() {
	// Main context and cancellation tokens
	var (
		ctx    context.Context
		cancel context.CancelFunc
		tp     *trace.TracerProvider
		err    error
	)

	// Initialize configuration
	appConfig := configuration.AppConfig()

	// Configure command-line parameters
	pflag.Int32VarP(&appConfig.CleanupTimeoutSec, "timeout", "t", 60, "Time to wait for graceful shutdown on SIGTERM/SIGINT in seconds. Default: 60")
	pflag.Int32VarP(&appConfig.HttpPort, "port", "p", 8080, "TCP port for the HTTP listener to bind to. Default: 8080")
	pflag.BoolVarP(&appConfig.UseSwagger, "swagger", "s", false, "Activate swagger. Do not use this in Production!")
	pflag.BoolVarP(&appConfig.Development, "devel", "d", false, "Start in development mode. Implies --swagger. Do not use this in Production!")
	pflag.BoolVarP(&appConfig.GinLogger, "gin-logger", "g", false, "Activate Gin's logger, for debugging. Do not use this in Production!")
	pflag.StringVarP(&appConfig.UseTelemetry, "telemetry", "r", "", "Activate telemetry local or remote/jaeger")
	pflag.BoolVarP(&appConfig.PprofEnabled, "pprof", "o", false, "Enable pprof endpoints")
	pflag.Parse()

	// Initialize main context and set up cancellation token for SIGINT/SIGQUIT
	ctx = context.Background()
	ctx, cancel = context.WithCancel(ctx)
	cSignal := make(chan os.Signal, 1)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Initialize logger
	logger.Init(ctx, true, appConfig.Development)
	logger.SetCorrelationIdFieldKey(configuration.CorrelationIdKey)
	logger.SetCorrelationIdContextKey(configuration.CorrelationIdKey)
	log := logger.SugaredLogger()
	//goland:noinspection GoUnhandledErrorResult
	defer log.Sync()
	defer logger.PanicLogger()

	// setting max procs limits
	_, err = maxprocs.Set(maxprocs.Logger(log.Printf))
	if err != nil {
		log.Fatalf("failed to set GOMAXPROCS: %v", err)
	}
	log.Debugf("Running with GOMAXPROCS: %d", runtime.GOMAXPROCS(0))

	// Sanity checks
	if !appConfig.Development {
		if appConfig.CleanupTimeoutSec < 120 {
			log.Warnf("Cleanup timeout is set to %d seconds which might be too small for production mode!", appConfig.CleanupTimeoutSec)
		}
	}

	if appConfig.Development {
		appConfig.UseSwagger = true
	}

	if appConfig.UseSwagger {
		// set swagger from yaml appConfig
		appConfig.LoadSwaggerConf()
		docs.SwaggerInfo.Title = appConfig.Swagger.Title
		docs.SwaggerInfo.Version = appConfig.Swagger.Version
		docs.SwaggerInfo.BasePath = appConfig.IngressPrefix + appConfig.Swagger.BasePath
		docs.SwaggerInfo.Description = appConfig.Swagger.Description
	}
	log.Infof(docs.SwaggerInfo.BasePath)

	// Telemetry
	if appConfig.JaegerEndpoint != "" && appConfig.UseTelemetry == "" {
		appConfig.UseTelemetry = "remote"
	}

	// Initialize Magika scanner (ONNX runtime)
	log.Infof("Initializing Magika scanner")
	if err = services.InitMagikaScanner(appConfig); err != nil {
		log.Fatalf("error initializing Magika scanner: %v", err)
	}
	log.Infof("Magika scanner initialized successfully")

	switch appConfig.UseTelemetry {
	case "remote":
		log.Infof("Jaeger Telemetry enabled")
		// init tracer jaeger
		tp, err = tracer.InitTracerJaeger(ctx, appConfig.JaegerEndpoint, configuration.OTName, configuration.OTName, appConfig.Environment)
		if err != nil {
			log.Fatal(err)
		}
	case "local":
		log.Infof("stdout Telemetry enabled")
		// init tracer jaeger
		tp, err = tracer.InitTracerStdout(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Trigger context cancellation token on SIGINT/SIGTERM
	go func() {
		<-cSignal
		log.Warnf("SIGTERM received, attempting graceful exit.")
		cancel()
	}()

	// Start the API HTTP Server
	log.Info("Starting webapi handler")
	concurrency.GlobalWaitGroup.Add(1)
	go api.StartGin(ctx)

	// Block until cancellation signal is received
	<-ctx.Done()

	// Clean up and attempt graceful exit
	log.Infof("graceful shutdown initiated. Waiting for %d seconds before forced exit.", appConfig.CleanupTimeoutSec)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*time.Duration(appConfig.CleanupTimeoutSec))
	go func() {
		// Eventual clean-up logic would go in this block
		if tp != nil {
			concurrency.GlobalWaitGroup.Add(1)
			go func() {
				defer concurrency.GlobalWaitGroup.Done()
				log.Debugf("shutting down telemetry provider")
				if err = tp.Shutdown(context.Background()); err != nil {
					log.Errorf("error shutting down tracer provider: %v", err)
				}
				log.Debugf("telemetry provider terminated")
			}()
		}
		concurrency.GlobalWaitGroup.Wait()
		log.Infof("cleanup done.")
		cancel()
	}()
	<-ctx.Done()
	log.Info("exiting.")
}
