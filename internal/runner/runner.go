package runner

import (
	"context"
	"docker-hytale-server/internal/config"
	"docker-hytale-server/internal/oauth"
	"docker-hytale-server/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/fatih/color"
)

type Defaults struct {
	World    string `json:"World"`
	GameMode string `json:"GameMode"`
}
type PlayerStorage struct {
	Type string `json:"Type"`
}
type ServerConfig struct {
	Version                 int            `json:"Version"`
	ServerName              string         `json:"ServerName"`
	MOTD                    string         `json:"MOTD"`
	Password                string         `json:"Password"`
	MaxPlayers              int            `json:"MaxPlayers"`
	MaxViewRadius           int            `json:"MaxViewRadius"`
	Defaults                Defaults       `json:"Defaults"`
	ConnectionTimeouts      map[string]any `json:"ConnectionTimeouts"`
	RateLimit               map[string]any `json:"RateLimit"`
	Modules                 map[string]any `json:"Modules"`
	LogLevels               map[string]any `json:"LogLevels"`
	Mods                    map[string]any `json:"Mods"`
	DisplayTmpTagsInStrings bool           `json:"DisplayTmpTagsInStrings"`
	PlayerStorage           PlayerStorage  `json:"PlayerStorage"`
	Update                  map[string]any `json:"Update"`
	Backup                  map[string]any `json:"Backup"`
}

func createConfigJson() {
	logPrefix := "[CREATE-CONFIG-JSON]"

	cfg := config.Get()

	log.WithPrefix(logPrefix).Info("Creating the 'config.json' file...")

	serverConfig := ServerConfig{
		Version:       4,
		ServerName:    cfg.ServerName,
		MOTD:          cfg.ServerMotd,
		Password:      cfg.ServerPassword,
		MaxPlayers:    cfg.MaxPlayers,
		MaxViewRadius: cfg.MaxViewRadius,
		Defaults: Defaults{
			World:    cfg.DefaultWorld,
			GameMode: cfg.DefaultGameMode,
		},
		ConnectionTimeouts:      make(map[string]any),
		RateLimit:               make(map[string]any),
		Modules:                 make(map[string]any),
		LogLevels:               make(map[string]any),
		Mods:                    make(map[string]any),
		DisplayTmpTagsInStrings: cfg.DisplayTmpTagsInStrings,
		PlayerStorage: PlayerStorage{
			Type: cfg.PlayerStorageType,
		},
		Update: make(map[string]any),
		Backup: make(map[string]any),
	}

	jsonData, errMarshal := json.MarshalIndent(serverConfig, "", "  ")
	if errMarshal != nil {
		log.WithPrefix(logPrefix).Errorf("Error encoding the 'ServerConfig' structure to json. Error - [ %s ]", errMarshal)
		return
	}

	if errWriteFile := os.WriteFile(cfg.ConfigServerFile, jsonData, 0644); errWriteFile != nil {
		log.WithPrefix(logPrefix).Errorf("The config.json file was not created. Error - [ %s ]", errWriteFile)
		return
	}

	log.WithPrefix(logPrefix).Info(" ✅ The config.json file was successfully created!")
}

func LogIntro() {
	fmt.Println("")
	color.Magenta("=============================================")
	fmt.Println("")
	color.Magenta("Hytale-Server")
	fmt.Println("")
	color.Magenta("=============================================")
	fmt.Println("")
}

func isServerFilesExist() bool {
	cfg := config.Get()
	return utils.CheckFileExists(cfg.ServerJar) && utils.CheckFileExists(cfg.AssetsFile)
}

func buildJavaArgs(gameSession *oauth.GameSession) []string {
	logPrefix := "[BUILD-JAVA-ARGS]"

	args := make([]string, 0, 50)

	cfg := config.Get()

	args = append(args, "-Xms"+cfg.JavaXms, "-Xmx"+cfg.JavaXmx)

	if cfg.EnableAotCache && utils.CheckFileExists(cfg.AotCache) {
		args = append(args, "-XX:AOTCache="+cfg.AotCache)
	}

	// ==========================================================================
	// Aikar's flags: https://docs.papermc.io/paper/aikars-flags
	// ==========================================================================
	args = append(args,
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:G1NewSizePercent=30",
		"-XX:G1MaxNewSizePercent=40",
		"-XX:G1HeapRegionSize=8M",
		"-XX:G1ReservePercent=20",
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:InitiatingHeapOccupancyPercent=15",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
	)

	if cfg.JavaOpts != "" {
		javaOpts := strings.Fields(cfg.JavaOpts)
		args = append(args, javaOpts...)
	}

	args = append(args, "-jar", cfg.ServerJar)

	args = append(args, "--assets", cfg.AssetsFile)

	args = append(args, "--auth-mode", cfg.AuthMode)

	args = append(args, "--bind", fmt.Sprintf("%s:%s", cfg.BindAddress, cfg.ServerPort))

	if cfg.DisableSentry {
		args = append(args, "--disable-sentry")
	}

	if cfg.AllowOp {
		args = append(args, "--allow-op")
	}

	if cfg.AcceptEarlyPlugins {
		log.WithPrefix(logPrefix).Warn(" ⚠️ Early plugins enabled - this is unsupported and may cause stability issues")
		args = append(args, "--accept-early-plugins")
	}

	if cfg.EnableBackups {
		args = append(args, "--backup",
			"--backup-dir", cfg.BackupDir,
			"--backup-frequency", cfg.BackupFrequency,
			"--backup-max-count", cfg.BackupMaxCount,
		)

		log.WithPrefix(logPrefix).Infof("Backups enabled: every %s minutes to %s (max %s)", cfg.BackupFrequency, cfg.BackupDir, cfg.BackupMaxCount)
	}

	if cfg.ServerLogLevel != "" {
		args = append(args, "--log", cfg.ServerLogLevel)
	}

	if gameSession != nil {
		args = append(args,
			"--session-token", gameSession.SessionToken,
			"--identity-token", gameSession.IdentityToken,
			"--owner-uuid", gameSession.ProfileUUID,
		)

		if cfg.HytaleOwnerName != "" {
			args = append(args, "--owner-name", cfg.HytaleOwnerName)
		}
	}

	return args
}

func DownloadServerFiles(ctxSignal context.Context) {
	logPrefix := "[HYTALE-DOWNLOADER-CLI]"

	cfg := config.Get()

	if isServerFilesExist() {
		log.WithPrefix(logPrefix).Info("The server files already exist. Skip this step.")
		return
	}

	if err := utils.CreateDirectories(cfg.DataDir, cfg.OAuthDir); err != nil {
		log.WithPrefix(logPrefix).Fatal(err)
	}

	downloadPath := filepath.Join(cfg.DataDir, "game.zip")
	arg := []string{"-download-path", downloadPath, "-patchline", cfg.HytalePatchline, "-credentials-path", cfg.DownloaderCredentialsFile}

	cmd := exec.CommandContext(ctxSignal, cfg.HytaleDownloaderCli, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	title := "HYTALE DOWNLOADER AUTHENTICATION REQUIRED"
	if utils.CheckFileExists(cfg.DownloaderCredentialsFile) {
		title = "HYTALE DOWNLOADER"
	}

	if err := cmd.Start(); err != nil {
		log.WithPrefix(logPrefix).Fatalf("Error starting Hytale Downloader CLI. %s", err)
	}

	log.WithPrefix(logPrefix).Info("Downloading server files using Hytale Downloader CLI...")

	fmt.Println("=============================================")
	fmt.Println(title)
	fmt.Println("=============================================")
	if err := cmd.Wait(); err != nil {
		if ctxSignal.Err() != nil {
			log.WithPrefix(logPrefix).Fatal("The process was interrupted")
		}

		reasons := []string{
			" ❗ Download failed!",
			"This could be due to invalid/expired OAuth credentials.",
			"Please try to delete the '.hytale-downloader-credentials.json' file, start the container and re-auth again.",
			fmt.Sprintf("Error: %s", err),
		}
		log.WithPrefix(logPrefix).Fatal(strings.Join(reasons, "\n"))
	}

	log.WithPrefix(logPrefix).Info("The download is finished")

	unzipFile(ctxSignal, downloadPath, cfg.DataDir)

	log.WithPrefix(logPrefix).Info("Cleanup downloaded a zip archive...")

	if err := os.RemoveAll(downloadPath); err != nil {
		log.WithPrefix(logPrefix).Errorf("Error deleting a zip archive on the path - %s: %s", downloadPath, err)
	} else {
		log.WithPrefix(logPrefix).Info("The cleaning was successful")
	}
}

func unzipFile(ctxSignal context.Context, zipPath, targetDir string) {
	logPrefix := "[UNZIP]"

	log.WithPrefix(logPrefix).Info("Extracting a zip archive...")

	cmd := exec.CommandContext(ctxSignal, "unzip", "-q", "-o", zipPath, "-d", targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.WithPrefix(logPrefix).Fatalf("Unzip failed: %s", err)
	}

	log.WithPrefix(logPrefix).Info("The extraction of a zip archive was successful!")
}

func StartHytaleServer(ctxSignal context.Context, gameSession *oauth.GameSession) {
	logPrefix := "[HYTALE-SERVER]"

	cfg := config.Get()

	log.WithPrefix(logPrefix).Info("Starting the hytale server...")

	javaArgs := buildJavaArgs(gameSession)

	cmd := exec.CommandContext(ctxSignal, "java", javaArgs...)

	cmd.Dir = cfg.DataDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if ctxSignal.Err() != nil {
		log.WithPrefix(logPrefix).Info("Stopping the hytale server...")
		return
	}

	if err != nil {
		log.WithPrefix(logPrefix).Fatalf("The hytale server error has occurred: %s", err)
	}
}

func SettingsConfigJson() {
	logPrefix := "[SETTINGS-CONFIG-JSON]"

	cfg := config.Get()

	file, err := os.Open(cfg.ConfigServerFile)

	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			log.WithPrefix(logPrefix).Warn(" ⚠️ The config.json file doesn't exist. Let's create it!")
			createConfigJson()
		case errors.Is(err, os.ErrPermission):
			log.WithPrefix(logPrefix).Warn(" ⚠️ Insufficient permission to read. Skip this step.")
		default:
			log.WithPrefix(logPrefix).Errorf("An unexpected error has occurred. Skip this step. Error - [ %s ]", err)
		}

		return
	}

	var serverConfig ServerConfig
	errDecode := json.NewDecoder(file).Decode(&serverConfig)
	file.Close()

	if errDecode != nil {
		log.WithPrefix(logPrefix).Errorf("Error decoding the config.json file into a structure. Skip this step. Error - [ %s ]", errDecode)
		return
	}

	isChanged := false

	if serverConfig.ServerName != cfg.ServerName {
		serverConfig.ServerName = cfg.ServerName

		isChanged = true
	}

	if serverConfig.MOTD != cfg.ServerMotd {
		serverConfig.MOTD = cfg.ServerMotd

		isChanged = true
	}

	if serverConfig.Password != cfg.ServerPassword {
		serverConfig.Password = cfg.ServerPassword

		isChanged = true
	}

	if serverConfig.MaxPlayers != cfg.MaxPlayers {
		serverConfig.MaxPlayers = cfg.MaxPlayers

		isChanged = true
	}

	if serverConfig.MaxViewRadius != cfg.MaxViewRadius {
		serverConfig.MaxViewRadius = cfg.MaxViewRadius

		isChanged = true
	}

	if serverConfig.Defaults.World != cfg.DefaultWorld {
		serverConfig.Defaults.World = cfg.DefaultWorld

		isChanged = true
	}

	if serverConfig.Defaults.GameMode != cfg.DefaultGameMode {
		serverConfig.Defaults.GameMode = cfg.DefaultGameMode

		isChanged = true
	}

	if serverConfig.DisplayTmpTagsInStrings != cfg.DisplayTmpTagsInStrings {
		serverConfig.DisplayTmpTagsInStrings = cfg.DisplayTmpTagsInStrings

		isChanged = true
	}

	if serverConfig.PlayerStorage.Type != cfg.PlayerStorageType {
		serverConfig.PlayerStorage.Type = cfg.PlayerStorageType

		isChanged = true
	}

	if !isChanged {
		log.WithPrefix(logPrefix).Info("No changes detected")
		return
	}

	log.WithPrefix(logPrefix).Info("Changes detected")

	jsonData, errMarshal := json.MarshalIndent(serverConfig, "", "  ")
	if errMarshal != nil {
		log.WithPrefix(logPrefix).Errorf("Error encoding the 'ServerConfig' structure to json. Error - [ %s ]", errMarshal)
		return
	}

	if errWriteFile := os.WriteFile(cfg.ConfigServerFile, jsonData, 0644); errWriteFile != nil {
		log.WithPrefix(logPrefix).Errorf("The config.json file was not updated. Error - [ %s ]", errWriteFile)
		return
	}

	log.WithPrefix(logPrefix).Info(" ✅ The config.json file was successfully updated!")
}
