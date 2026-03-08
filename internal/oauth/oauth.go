package oauth

import (
	"bytes"
	"context"
	"docker-hytale-server/internal/config"
	"docker-hytale-server/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fatih/color"
)

const deviceUrl = "https://oauth.accounts.hytale.com/oauth2/device/auth"
const tokenUrl = "https://oauth.accounts.hytale.com/oauth2/token"
const profilesUrl = "https://account-data.hytale.com/my-account/get-profiles"
const sessionUrl = "https://sessions.hytale.com/game-session/new"

const clientId = "hytale-server"
const scopes = "openid offline auth:server"

type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

type Profile struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

type ProfilesResponse struct {
	Owner    string    `json:"owner"`
	Profiles []Profile `json:"profiles"`
}

type SessionTokensResponse struct {
	SessionToken  string `json:"sessionToken"`
	IdentityToken string `json:"identityToken"`
	ExpiresAt     string `json:"expiresAt"`
}

type GameSession struct {
	SessionToken  string
	IdentityToken string
	ProfileUUID   string
	ExpiresAt     string
}

type TokensStorage struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshedAt  time.Time `json:"refreshed_at"`
}

var client = http.Client{}

func isOK(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func fetch(req *http.Request, result any, logPrefix string) (int, error) {
	res, err := client.Do(req)

	if err != nil {
		return 0, fmt.Errorf("error sending request. %w", err)
	}

	defer res.Body.Close()

	log.WithPrefix(logPrefix).Infof("Method - %s: The '%s' request ended with status code %d", req.Method, req.URL, res.StatusCode)

	if decodingErr := json.NewDecoder(res.Body).Decode(result); decodingErr != nil {
		return res.StatusCode, fmt.Errorf("error in decoding the body. %w", decodingErr)
	}

	return res.StatusCode, nil
}

func isAccessTokenExpired(expiresAt time.Time) bool {
	if expiresAt.IsZero() {
		return true
	}

	nowWithBuffer := time.Now().UTC().Add(60 * time.Second)

	return nowWithBuffer.After(expiresAt)
}

func loadOAuthTokens() *TokensStorage {
	logPrefix := "[LOAD-OAUTH-TOKENS]"

	cfg := config.Get()

	content, errRead := os.ReadFile(cfg.ServerCredentialsFile)
	if errRead != nil {
		log.WithPrefix(logPrefix).Errorf("Failed to read the .hytale-server-credentials.json file. %s", errRead)
		return nil
	}

	var oauthTokens TokensStorage

	if errUnmarshal := json.Unmarshal(content, &oauthTokens); errUnmarshal != nil {
		log.WithPrefix(logPrefix).Errorf("Error decoding the .hytale-server-credentials.json file into a structure. %s", errUnmarshal)
		return nil
	}

	if oauthTokens.RefreshToken == "" {
		log.WithPrefix(logPrefix).Error("No refresh token available.")
		return nil
	}

	return &oauthTokens
}

func saveOAuthTokens(tokens *TokensResponse) {
	logPrefix := "[SAVE-OAUTH-TOKENS]"

	cfg := config.Get()

	if err := utils.CreateDirectories(cfg.OAuthDir); err != nil {
		log.WithPrefix(logPrefix).Error(err)
		return
	}

	data := TokensStorage{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		RefreshedAt:  time.Now().UTC(),
	}

	jsonData, errMarshal := json.MarshalIndent(data, "", "  ")
	if errMarshal != nil {
		log.WithPrefix(logPrefix).Errorf("Error encoding the 'TokenStorage' structure to json. %s", errMarshal)
		return
	}

	if err := os.WriteFile(cfg.ServerCredentialsFile, jsonData, 0600); err != nil {
		log.WithPrefix(logPrefix).Errorf("Error saving tokens to file. %s", err)
	}
}

func getAccessToken(ctxSignal context.Context) string {
	logPrefix := "[GET-ACCESS-TOKEN]"
	log.WithPrefix(logPrefix).Info("Trying to get access token...")

	oauthTokens := loadOAuthTokens()
	if oauthTokens == nil {
		log.WithPrefix(logPrefix).Warn(" ⚠️ Failed to load oauth tokens")
		return ""
	}

	if isAccessTokenExpired(oauthTokens.ExpiresAt) {
		log.WithPrefix(logPrefix).Info("Access token is expired, starting refreshing...")
		updatedTokens := refreshOAuthTokens(ctxSignal)

		if updatedTokens != nil {
			log.WithPrefix(logPrefix).Info("Access token received")
			return updatedTokens.AccessToken
		}

		log.WithPrefix(logPrefix).Warn(" ⚠️ Failed to refresh access token")
		return ""
	}

	log.WithPrefix(logPrefix).Info("Access token received")
	return oauthTokens.AccessToken
}

func logDeviceAuthorization(deviceRes *DeviceAuthResponse) {
	fmt.Println("=============================================")
	fmt.Println("HYTALE SERVER AUTHENTICATION REQUIRED")
	fmt.Println("=============================================")
	fmt.Printf("Visit: %s\n", deviceRes.VerificationUri)
	fmt.Printf("Enter code: %s\n", deviceRes.UserCode)
	fmt.Printf("Or visit: %s\n", deviceRes.VerificationUriComplete)
	fmt.Println("=============================================")
	fmt.Printf("Waiting for authorization (expires in %d seconds)...\n", deviceRes.ExpiresIn)
	fmt.Println("")
}

func startDeviceAuth(ctxSignal context.Context) *DeviceAuthResponse {
	logPrefix := "[START-DEVICE-AUTH]"

	var result DeviceAuthResponse

	formData := url.Values{
		"client_id": {clientId},
		"scope":     {scopes},
	}

	body := strings.NewReader(formData.Encode())

	req, err := http.NewRequestWithContext(ctxSignal, http.MethodPost, deviceUrl, body)
	if err != nil {
		log.WithPrefix(logPrefix).Errorf("Error creating request. %s", err)
		return nil
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	statusCode, errFetch := fetch(req, &result, logPrefix)

	if ctxSignal.Err() != nil {
		log.WithPrefix(logPrefix).Fatal("The process was interrupted")
	}

	if errFetch != nil {
		log.WithPrefix(logPrefix).Error(errFetch)
		return nil
	}

	if !isOK(statusCode) {
		return nil
	}

	return &result
}

func pollForToken(ctxSignal context.Context, deviceAuthResponse *DeviceAuthResponse) *TokensResponse {
	logPrefix := "[POLL-FOR-TOKEN]"

	expiresIn := time.Duration(deviceAuthResponse.ExpiresIn) * time.Second
	interval := time.Duration(deviceAuthResponse.Interval) * time.Second

	ctxTimeout, cancel := context.WithTimeout(context.Background(), expiresIn)
	defer cancel()

	fetchToken := func() *TokensResponse {
		var result TokensResponse

		formData := url.Values{
			"client_id":   {clientId},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {deviceAuthResponse.DeviceCode},
		}

		body := strings.NewReader(formData.Encode())

		req, err := http.NewRequestWithContext(ctxSignal, http.MethodPost, tokenUrl, body)
		if err != nil {
			log.WithPrefix(logPrefix).Errorf("Error creating request. %s", err)
			return nil
		}

		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		_, errFetch := fetch(req, &result, logPrefix)

		if ctxSignal.Err() != nil {
			log.WithPrefix(logPrefix).Fatal("The process was interrupted")
		}

		if errFetch != nil {
			log.WithPrefix(logPrefix).Error(errFetch)
			return nil
		}

		return &result
	}

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {

		timer.Reset(interval)

		select {
		case <-ctxSignal.Done():
			log.WithPrefix(logPrefix).Fatal("The process was interrupted")
		case <-ctxTimeout.Done():
			log.WithPrefix(logPrefix).Warn(" ⚠️ The authorization time has expired.")
			return nil
		case <-timer.C:
			result := fetchToken()

			if result == nil {
				return nil
			}

			switch result.Error {
			case "authorization_pending":
				log.WithPrefix(logPrefix).Info("Authorization pending...")
				continue
			case "slow_down":
				interval += 5 * time.Second
				log.WithPrefix(logPrefix).Warnf(" ⚠️ Slow down. The interval before repeating the request is now %s.", interval)
				continue
			case "expired_token":
				log.WithPrefix(logPrefix).Error("Device code expired.")
				return nil
			case "access_denied":
				log.WithPrefix(logPrefix).Error("User denied authorization.")
				return nil
			case "":
				log.WithPrefix(logPrefix).Info(color.GreenString("Authorization successful!"))
				saveOAuthTokens(result)
				return result
			default:
				log.WithPrefix(logPrefix).Error(result.Error)
				return nil
			}
		}
	}
}

func refreshOAuthTokens(ctxSignal context.Context) *TokensResponse {
	logPrefix := "[REFRESH-OAUTH-TOKENS]"

	log.WithPrefix(logPrefix).Info("Refreshing OAuth tokens...")

	storedTokens := loadOAuthTokens()
	if storedTokens == nil {
		log.WithPrefix(logPrefix).Warn(" ⚠️ Cancel. Failed to get refresh token")
		return nil
	}

	var result TokensResponse

	formData := url.Values{
		"client_id":     {clientId},
		"grant_type":    {"refresh_token"},
		"refresh_token": {storedTokens.RefreshToken},
	}

	body := strings.NewReader(formData.Encode())

	req, err := http.NewRequestWithContext(ctxSignal, http.MethodPost, tokenUrl, body)
	if err != nil {
		log.WithPrefix(logPrefix).Errorf("Error creating request. %s", err)
		return nil
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	_, errFetch := fetch(req, &result, logPrefix)

	if ctxSignal.Err() != nil {
		log.WithPrefix(logPrefix).Fatal("The process was interrupted")
	}

	if errFetch != nil {
		log.WithPrefix(logPrefix).Error(errFetch)
		return nil
	}

	if result.Error != "" {
		log.WithPrefix(logPrefix).Errorf("Token refresh failed: %s", result.Error)
		return nil
	}

	log.WithPrefix(logPrefix).Info("OAuth tokens refreshed successfully!")
	saveOAuthTokens(&result)
	return &result
}

func getGameProfiles(ctxSignal context.Context, accessToken string) *Profile {
	logPrefix := "[GET-GAME-PROFILES]"

	var result ProfilesResponse

	req, err := http.NewRequestWithContext(ctxSignal, http.MethodGet, profilesUrl, nil)
	if err != nil {
		log.WithPrefix(logPrefix).Errorf("Error creating request. %s", err)
		return nil
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	statusCode, errFetch := fetch(req, &result, logPrefix)

	if ctxSignal.Err() != nil {
		log.WithPrefix(logPrefix).Fatal("The process was interrupted")
	}

	if errFetch != nil {
		log.WithPrefix(logPrefix).Error(errFetch)
		return nil
	}

	if !isOK(statusCode) {
		return nil
	}

	if len(result.Profiles) <= 0 {
		log.WithPrefix(logPrefix).Warn(" ⚠️ No game profiles found")
		return nil
	}

	profile := result.Profiles[0]

	log.WithPrefix(logPrefix).Infof("Profile: username: %s, uuid: %s", profile.Username, profile.UUID)

	return &profile
}

func createGameSession(ctxSignal context.Context, uuid, accessToken string) *GameSession {
	logPrefix := "[CREATE-GAME-SESSION]"

	var result SessionTokensResponse

	data := map[string]any{
		"uuid": uuid,
	}

	jsonData, err := json.Marshal(data)

	if err != nil {
		log.WithPrefix(logPrefix).Errorf("Error encoding a map to json. %s", err)
		return nil
	}

	body := bytes.NewBuffer(jsonData)

	req, err := http.NewRequestWithContext(ctxSignal, http.MethodPost, sessionUrl, body)
	if err != nil {
		log.WithPrefix(logPrefix).Errorf("Error creating request. %s", err)
		return nil
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/json")

	statusCode, errFetch := fetch(req, &result, logPrefix)

	if ctxSignal.Err() != nil {
		log.WithPrefix(logPrefix).Fatal("The process was interrupted")
	}

	if errFetch != nil {
		log.WithPrefix(logPrefix).Error(errFetch)
		return nil
	}

	if !isOK(statusCode) {
		return nil
	}

	log.WithPrefix(logPrefix).Infof("Session created (expires: %s)", result.ExpiresAt)
	return &GameSession{result.SessionToken, result.IdentityToken, uuid, result.ExpiresAt}
}

func GetGameSession(ctxSignal context.Context) *GameSession {
	logPrefix := "[GET-GAME-SESSION]"

	cfg := config.Get()

	// For hosting providers who want to programmatically manage credentials and persist tokens across server restarts via plugins.
	if utils.IsNonEmptyString(cfg.HytaleServerSessionToken, cfg.HytaleServerIdentityToken, cfg.HytaleOwnerUUID) {
		log.WithPrefix(logPrefix).Info("Using session tokens from environment variables")
		return &GameSession{
			SessionToken:  cfg.HytaleServerSessionToken,
			IdentityToken: cfg.HytaleServerIdentityToken,
			ProfileUUID:   cfg.HytaleOwnerUUID,
			ExpiresAt:     "",
		}
	}

	accessToken := getAccessToken(ctxSignal)

	if accessToken != "" {
		log.WithPrefix(logPrefix).Info("Skipping authentication. Access token available.")
		profile := getGameProfiles(ctxSignal, accessToken)
		if profile != nil {
			return createGameSession(ctxSignal, profile.UUID, accessToken)
		}
		return nil
	}

	log.WithPrefix(logPrefix).Info("Starting authentication...")
	deviceAuthResponse := startDeviceAuth(ctxSignal)
	if deviceAuthResponse == nil {
		return nil
	}

	logDeviceAuthorization(deviceAuthResponse)

	tokenResponse := pollForToken(ctxSignal, deviceAuthResponse)
	if tokenResponse == nil {
		return nil
	}

	profile := getGameProfiles(ctxSignal, tokenResponse.AccessToken)
	if profile == nil {
		return nil
	}

	gameSession := createGameSession(ctxSignal, profile.UUID, tokenResponse.AccessToken)

	return gameSession
}

func AutoRefreshTokens(ctxSignal context.Context, wg *sync.WaitGroup) {
	logPrefix := "[AUTO-REFRESH-TOKENS]"
	defer wg.Done()

	interval := 60 * time.Second

	timer := time.NewTimer(interval)
	defer timer.Stop()

	checkAndRefreshOAuth := func() {
		oauthTokens := loadOAuthTokens()
		if oauthTokens == nil {
			log.WithPrefix(logPrefix).Warn(" ⚠️ Failed to load oauth tokens")
			return
		}

		if isAccessTokenExpired(oauthTokens.ExpiresAt) {
			log.WithPrefix(logPrefix).Info("Access token is expired, starting refreshing...")
			refreshOAuthTokens(ctxSignal)
		}
	}

	for {
		timer.Reset(interval)

		select {
		case <-ctxSignal.Done():
			log.WithPrefix(logPrefix).Info(" ✅ Automatic refresh tokens was successfully stopped.")
			return
		case <-timer.C:
			checkAndRefreshOAuth()
		}
	}
}
