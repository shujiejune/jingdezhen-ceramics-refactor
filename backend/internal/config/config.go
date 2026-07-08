package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config holds every external integration secret + tunable the app needs.
// Sandbox/live is a per-adapter env flip (TDD §4.1): each external service has
// its own `*_ENV` ("sandbox"|"live"); mock impls are selected in code when the
// live service isn't onboarded yet. Never commit real secrets — keep them in
// the gitignored .env (see .env.example for the full key list).
type Config struct {
	// --- Server ---
	ServerPort string `mapstructure:"SERVER_PORT"`
	ClientOrigin string `mapstructure:"CLIENT_ORIGIN"`
	JWTSecret    string `mapstructure:"JWT_SECRET"`
	AdminEmail   string `mapstructure:"ADMIN_EMAIL"` // inbox for admin notifications

	// --- Database (PostgreSQL) ---
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// --- Redis (sessions, cache, Asynq queues, pub/sub) ---
	// Single URL encodes host/port/db/password, e.g. redis://:pass@localhost:6379/0
	RedisURL string `mapstructure:"REDIS_URL"`

	// --- Google OAuth ---
	GoogleOAuthClientID     string `mapstructure:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecret string `mapstructure:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleOAuthRedirectURL  string `mapstructure:"GOOGLE_OAUTH_REDIRECT_URL"`

	// --- Brevo (transactional email) — replaces AWS SES (TDD §10) ---
	BrevoAPIKey      string `mapstructure:"BREVO_API_KEY"`
	BrevoSenderEmail string `mapstructure:"BREVO_SENDER_EMAIL"` // verified sender
	BrevoSenderName  string `mapstructure:"BREVO_SENDER_NAME"`

	// --- Airwallex (card payments; settles to CNY) — sandbox until onboarding (PRD §7) ---
	AirwallexClientID     string `mapstructure:"AIRWALLEX_CLIENT_ID"`
	AirwallexAPIKey       string `mapstructure:"AIRWALLEX_API_KEY"`
	AirwallexEnv          string `mapstructure:"AIRWALLEX_ENV"` // "sandbox" | "live"
	AirwallexWebhookSecret string `mapstructure:"AIRWALLEX_WEBHOOK_SECRET"`

	// --- PayPal (alternative payment) — sandbox until onboarding (PRD §7) ---
	PayPalClientID     string `mapstructure:"PAYPAL_CLIENT_ID"`
	PayPalClientSecret string `mapstructure:"PAYPAL_CLIENT_SECRET"`
	PayPalEnv          string `mapstructure:"PAYPAL_ENV"` // "sandbox" | "live"
	PayPalWebhookID    string `mapstructure:"PAYPAL_WEBHOOK_ID"`

	// --- Qwen (LLM for the ceramic-art assistant chat) ---
	QwenAPIKey  string `mapstructure:"QWEN_API_KEY"`
	QwenBaseURL string `mapstructure:"QWEN_BASE_URL"` // DashScope endpoint
	QwenModel   string `mapstructure:"QWEN_MODEL"`    // e.g. qwen-plus

	// --- Alibaba Cloud OSS (HK) — object storage + on-the-fly image processing (TDD §2.1) ---
	OSSEndpoint        string `mapstructure:"OSS_ENDPOINT"`        // e.g. oss-cn-hongkong.aliyuncs.com
	OSSAccessKeyID     string `mapstructure:"OSS_ACCESS_KEY_ID"`
	OSSAccessKeySecret string `mapstructure:"OSS_ACCESS_KEY_SECRET"`
	OSSBucket          string `mapstructure:"OSS_BUCKET"`
	OSSRegion          string `mapstructure:"OSS_REGION"` // e.g. cn-hongkong
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env") // Name of config file (without extension)
	viper.SetConfigType("env") // Or "dotenv" or "json", "yaml" etc.

	viper.AutomaticEnv() // Read in environment variables that match

	err = viper.ReadInConfig() // Find and read the config file
	if err != nil {
		// Handle errors reading the config file, but allow it if it's just "not found"
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found.")
		} else {
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}
