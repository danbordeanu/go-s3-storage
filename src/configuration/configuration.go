package configuration

import "github.com/danbordeanu/go-utils"

type Configuration struct {
	Swagger CSwagger

	IngressHost   string
	IngressPrefix string

	// Dependencies
	JaegerEndpoint string

	// Configuration
	HttpPort int32

	// Internal settings
	CleanupTimeoutSec int32
	Environment       string
	UseTelemetry      string
	Development       bool
	GinLogger         bool
	UseSwagger        bool
	Initialized       bool

	// baseUrl page
	RequestBaseUrl string

	// Cors allow origins
	CorsAllowOrigins string

	// maxeventsper sec
	MaxEventsPerSec int32
	// maxburstsize
	MaxBurstSize int32

	// pprof
	PprofEnabled bool

	// storage
	StorageDirectory string

	// S3 Authentication
	S3AuthEnabled     bool
	S3AuthRegion      string
	S3AccessKeyID     string
	S3SecretAccessKey string

	// magika model path
	MagikaAssetsDir string
	MagikaModelName string

	// Web UI
	WebUIEnabled  bool
	WebUIPrefix   string
	SessionSecret string
	SessionTTL    int32 // seconds

	// Local Auth (bootstrap admin)
	LocalAuthEnabled  bool
	LocalAuthUsername string
	LocalAuthPassword string

	// Azure AD (future)
	AzureADEnabled      bool
	AzureADTenantID     string
	AzureADClientID     string
	AzureADClientSecret string

	// UI Settings
	UIObjectsPerPage    int32 // Number of objects per page in browser (default: 50)
	UIMaxObjectsPerPage int32 // Maximum allowed objects per page (default: 500)

	// Storage quota
	StorageQuotaBytes int64 // Storage quota in bytes (default: 0 for unlimited)
}

var appConfig Configuration

func AppConfig() *Configuration {
	if appConfig.Initialized == false {
		loadEnvironmentVariables()
		appConfig.Initialized = true
	}
	return &appConfig
}

// loadEnvironmentVariables load env variables
func loadEnvironmentVariables() {
	appConfig.Environment = utils.EnvOrDefault("ENVIRONMENT", "local")
	appConfig.JaegerEndpoint = utils.EnvOrDefault("JAEGER_ENDPOINT", "")
	appConfig.CleanupTimeoutSec = utils.EnvOrDefaultInt32("SHUTDOWN_TIMEOUT", 300)
	appConfig.IngressHost = utils.EnvOrDefault("INGRESS_HOST", "s3-storage")
	appConfig.IngressPrefix = utils.EnvOrDefault("INGRESS_PREFIX", "")
	// request base url
	appConfig.RequestBaseUrl = utils.EnvOrDefault("REQUEST_BASE_URL", "http://localhost:8080")
	// CORS allow origins
	appConfig.CorsAllowOrigins = utils.EnvOrDefault("CORS_ALLOW_ORIGINS", "Disabled")
	// limiter max events per sec
	appConfig.MaxEventsPerSec = utils.EnvOrDefaultInt32("MAX_EVENTS_PER_SEC", 100)
	// limiter max burst size
	appConfig.MaxBurstSize = utils.EnvOrDefaultInt32("MAX_BURST_SIZE", 120)
	// storage directory
	appConfig.StorageDirectory = utils.EnvOrDefault("STORAGE_DIRECTORY", "/data")
	// S3 authentication
	appConfig.S3AuthEnabled = utils.EnvOrDefault("S3_AUTH_ENABLED", "false") == "true"
	appConfig.S3AuthRegion = utils.EnvOrDefault("S3_AUTH_REGION", "us-east-1")
	appConfig.S3AccessKeyID = utils.EnvOrDefault("S3_ACCESS_KEY_ID", "")
	appConfig.S3SecretAccessKey = utils.EnvOrDefault("S3_SECRET_ACCESS_KEY", "")
	// magika model path
	appConfig.MagikaAssetsDir = utils.EnvOrDefault("MAGIKA_ASSETS_DIR", "/opt/magika/assets")
	appConfig.MagikaModelName = utils.EnvOrDefault("MAGIKA_MODEL_NAME", "standard_v3_3")

	// Web UI
	appConfig.WebUIEnabled = utils.EnvOrDefault("WEB_UI_ENABLED", "false") == "true"
	appConfig.WebUIPrefix = utils.EnvOrDefault("WEB_UI_PREFIX", "/ui")
	appConfig.SessionSecret = utils.EnvOrDefault("SESSION_SECRET", "change-me-to-32-byte-secret-key!")
	appConfig.SessionTTL = utils.EnvOrDefaultInt32("SESSION_TTL", 86400) // 24 hours

	// UI Settings
	appConfig.UIObjectsPerPage = utils.EnvOrDefaultInt32("UI_OBJECTS_PER_PAGE", 50)
	appConfig.UIMaxObjectsPerPage = utils.EnvOrDefaultInt32("UI_MAX_OBJECTS_PER_PAGE", 500)

	// Local Auth (bootstrap admin)
	appConfig.LocalAuthEnabled = utils.EnvOrDefault("LOCAL_AUTH_ENABLED", "true") == "true"
	appConfig.LocalAuthUsername = utils.EnvOrDefault("LOCAL_AUTH_USERNAME", "admin")
	appConfig.LocalAuthPassword = utils.EnvOrDefault("LOCAL_AUTH_PASSWORD", "changeme")

	// TODO: We can add more auth providers in the future, e.g. LDAP, OIDC, etc.
	// Azure AD (future)
	appConfig.AzureADEnabled = utils.EnvOrDefault("AZURE_AD_ENABLED", "false") == "true"
	appConfig.AzureADTenantID = utils.EnvOrDefault("AZURE_AD_TENANT_ID", "")
	appConfig.AzureADClientID = utils.EnvOrDefault("AZURE_AD_CLIENT_ID", "")
	appConfig.AzureADClientSecret = utils.EnvOrDefault("AZURE_AD_CLIENT_SECRET", "")

	// quota
	appConfig.StorageQuotaBytes = utils.EnvOrDefaultInt64("STORAGE_QUOTA_BYTES", 0) // 0 for unlimited
}
