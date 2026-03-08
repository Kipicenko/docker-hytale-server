package config

import (
	"docker-hytale-server/internal/utils"
	"path/filepath"

	"github.com/charmbracelet/log"
)

type Config struct {
	DataDir                   string
	ServerDir                 string
	OAuthDir                  string
	ServerCredentialsFile     string
	DownloaderCredentialsFile string
	ServerJar                 string
	AotCache                  string
	AssetsFile                string
	ConfigServerFile          string
	HytaleDownloaderCli       string
	HytalePatchline           string
	JavaXms                   string
	JavaXmx                   string
	JavaOpts                  string
	EnableAotCache            bool
	DisableSentry             bool
	AllowOp                   bool
	BindAddress               string
	ServerPort                string
	AuthMode                  string
	ServerName                string
	ServerMotd                string
	ServerPassword            string
	MaxPlayers                int
	MaxViewRadius             int
	DefaultWorld              string
	DefaultGameMode           string
	DisplayTmpTagsInStrings   bool
	PlayerStorageType         string
	HytaleServerSessionToken  string
	HytaleServerIdentityToken string
	HytaleOwnerUUID           string
	HytaleOwnerName           string
	EnableBackups             bool
	BackupDir                 string
	BackupFrequency           string
	BackupMaxCount            string
	AcceptEarlyPlugins        bool
	ServerLogLevel            string
	AutoRefreshTokens         bool
}

var config Config

func Get() *Config {
	return &config
}

func Load() {
	logPrefix := "[CONFIG]"
	log.WithPrefix(logPrefix).Info("Loading configuration...")

	dataPath := "/data"
	serverPath := filepath.Join(dataPath, "Server")
	oauthPath := filepath.Join(dataPath, ".auth")

	config = Config{
		DataDir:                   dataPath,
		ServerDir:                 serverPath,
		OAuthDir:                  oauthPath,
		ServerCredentialsFile:     filepath.Join(oauthPath, ".hytale-server-credentials.json"),
		DownloaderCredentialsFile: filepath.Join(oauthPath, ".hytale-downloader-credentials.json"),
		ServerJar:                 filepath.Join(serverPath, "HytaleServer.jar"),
		AotCache:                  filepath.Join(serverPath, "HytaleServer.aot"),
		AssetsFile:                filepath.Join(dataPath, "Assets.zip"),
		ConfigServerFile:          filepath.Join(dataPath, "config.json"),
		HytaleDownloaderCli:       utils.GetEnv("HYTALE_DOWNLOADER_CLI", "/opt/hytale/cli/hytale-downloader"),
		HytalePatchline:           utils.GetEnv("HYTALE_PATCHLINE", "release"),
		JavaXms:                   utils.GetEnv("JAVA_XMS", "4G"),
		JavaXmx:                   utils.GetEnv("JAVA_XMX", "4G"),
		JavaOpts:                  utils.GetEnv("JAVA_OPTS", ""),
		EnableAotCache:            utils.GetEnvBool("ENABLE_AOT_CACHE", true),
		DisableSentry:             utils.GetEnvBool("DISABLE_SENTRY", false),
		AllowOp:                   utils.GetEnvBool("ALLOW_OP", false),
		BindAddress:               utils.GetEnv("BIND_ADDRESS", "0.0.0.0"),
		ServerPort:                utils.GetEnv("SERVER_PORT", "5520"),
		AuthMode:                  utils.GetEnv("AUTH_MODE", "authenticated"),
		ServerName:                utils.GetEnv("SERVER_NAME", "Hytale Server"),
		ServerMotd:                utils.GetEnv("SERVER_MOTD", ""),
		ServerPassword:            utils.GetEnv("SERVER_PASSWORD", ""),
		MaxPlayers:                utils.GetEnvInt("MAX_PLAYERS", 100),
		MaxViewRadius:             utils.GetEnvInt("MAX_VIEW_RADIUS", 32),
		DefaultWorld:              utils.GetEnv("DEFAULT_WORLD", "default"),
		DefaultGameMode:           utils.GetEnv("DEFAULT_GAME_MODE", "Adventure"),
		DisplayTmpTagsInStrings:   utils.GetEnvBool("DISPLAY_TMP_TAGS_IN_STRINGS", false),
		PlayerStorageType:         utils.GetEnv("PLAYER_STORAGE_TYPE", "Hytale"),
		HytaleServerSessionToken:  utils.GetEnv("HYTALE_SERVER_SESSION_TOKEN", ""),
		HytaleServerIdentityToken: utils.GetEnv("HYTALE_SERVER_IDENTITY_TOKEN", ""),
		HytaleOwnerUUID:           utils.GetEnv("HYTALE_OWNER_UUID", ""),
		HytaleOwnerName:           utils.GetEnv("HYTALE_OWNER_NAME", ""),
		EnableBackups:             utils.GetEnvBool("ENABLE_BACKUPS", false),
		BackupDir:                 utils.GetEnv("BACKUP_DIR", filepath.Join(dataPath, "backups")),
		BackupFrequency:           utils.GetEnv("BACKUP_FREQUENCY", "30"),
		BackupMaxCount:            utils.GetEnv("BACKUP_MAX_COUNT", "5"),
		AcceptEarlyPlugins:        utils.GetEnvBool("ACCEPT_EARLY_PLUGINS", false),
		ServerLogLevel:            utils.GetEnv("SERVER_LOG_LEVEL", ""),
		AutoRefreshTokens:         utils.GetEnvBool("AUTO_REFRESH_TOKENS", true),
	}

	log.WithPrefix(logPrefix).Info("Done!")
}
