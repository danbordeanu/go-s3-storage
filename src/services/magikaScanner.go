package services

import (
	"s3-storage/configuration"

	"github.com/google/magika/go/magika"
)

var (
	MagikaScanner *magika.Scanner

	maxConcurrentScans = 4
	ScanSem            = make(chan struct{}, maxConcurrentScans)
)

// InitMagikaScanner initializes the Magika scanner
// It should be called once during application startup, after the configuration has been loaded
// Returns an error if the scanner could not be initialized (e.g. due to missing model files)
// Parameters:
// - appConfig: The application configuration containing the Magika model path and assets directory
// Returns:
// - error: An error if the scanner could not be initialized, or nil on success
// Example usage:
// err := services.InitMagikaScanner(appConfig)
//
//	if err != nil {
//	    log.Fatalf("Failed to initialize Magika scanner: %v", err)
//	}
func InitMagikaScanner(appConfig *configuration.Configuration) error {
	var err error
	MagikaScanner, err = magika.NewScanner(
		appConfig.MagikaAssetsDir,
		appConfig.MagikaModelName,
	)
	return err
}
