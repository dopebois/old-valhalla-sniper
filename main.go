package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
	"golang.org/x/net/http2"
)

// made by @y_ga

type Config struct {
	DiscordToken string `json:"discordToken"`
	Password     string `json:"password"`
	GuildID      string `json:"guildId"`
	Webhook      string `json:"webhook"`
}

// made by @y_ga

var (
	config      Config
	mfaToken    string
	savedTicket string
	guilds      = make(map[string]string)
	httpClient  *http.Client
	mu          sync.Mutex
)

// made by @y_ga

func init() {
	configData, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Println("Couldn't read config:", err)
		os.Exit(1)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		fmt.Println("Error parsing config:", err)
		os.Exit(1)
	}

	transport := &http2.Transport{}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// made by @y_ga

func main() {
	fmt.Println("Sniper started...")

	refreshMfaToken()

	go connectWebSocket()

	ticker := time.NewTicker(250 * time.Second)
	go func() {
		for range ticker.C {
			refreshMfaToken()
		}
	}()

	keepAliveTicker := time.NewTicker(60 * time.Minute)
	go func() {
		for range keepAliveTicker.C {
			makeRequest("HEAD", "/", nil, nil)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

// made by @y_ga

func makeRequest(method, path string, headers map[string]string, body []byte) ([]byte, error) {
	url := "https://canary.discord.com" + path
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0")
	req.Header.Set("Authorization", config.DiscordToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Super-Properties", "eyJvcyI6IldpbmRvd3MiLCJicm93c2VyIjoiRmlyZWZveCIsImRldmljZSI6IiIsInN5c3RlbV9sb2NhbGUiOiJ0ci1UUiIsImJyb3dzZXJfdXNlcl9hZ2VudCI6Ik1vemlsbGEvNS4wIChXaW5kb3dzIE5UIDEwLjA7IFdpbjY0OyB4NjQ7IHJ2OjEzMy4wKSBHZWNrby8yMDEwMDEwMSBGaXJlZm94LzEzMy4wIiwiYnJvd3Nlcl92ZXJzaW9uIjoiMTMzLjAiLCJvc192ZXJzaW9uIjoiMTAiLCJyZWZlcnJlciI6Imh0dHBzOi8vd3d3Lmdvb2dsZS5jb20vIiwicmVmZXJyaW5nX2RvbWFpbiI6Ind3dy5nb29nbGUuY29tIiwic2VhcmNoX2VuZ2luZSI6Imdvb2dsZSIsInJlZmVycmVyX2N1cnJlbnQiOiIiLCJyZWZlcnJpbmdfZG9tYWluX2N1cnJlbnQiOiIiLCJyZWxlYXNlX2NoYW5uZWwiOiJjYW5hcnkiLCJjbGllbnRfYnVpbGRfbnVtYmVyIjozNTYxNDAsImNsaWVudF9ldmVudF9zb3VyY2UiOm51bGwsImhhc19jbGllbnRfbW9kcyI6ZmFsc2V9")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// made by @y_ga

func refreshMfaToken() bool {
	mu.Lock()
	defer mu.Unlock()

	data, err := makeRequest("PATCH", fmt.Sprintf("/api/v7/guilds/%s/vanity-url", config.GuildID), nil, nil)
	if err != nil {
		return false
	}

	response := gjson.ParseBytes(data)
	code := response.Get("code").Int()

	if code == 60003 {
		savedTicket = response.Get("mfa.ticket").String()

		mfaData := map[string]interface{}{
			"ticket":   savedTicket,
			"mfa_type": "password",
			"data":     config.Password,
		}
		mfaBody, _ := json.Marshal(mfaData)

		mfaResp, err := makeRequest("POST", "/api/v9/mfa/finish", nil, mfaBody)
		if err != nil {
			return false
		}

		mfaToken = gjson.ParseBytes(mfaResp).Get("token").String()
		return mfaToken != ""
	} else if code == 200 {
		return true
	}

	return false
}

// made by @y_ga

func vanityUpdate(find string) {
	if mfaToken == "" {
		refreshMfaToken()
	}

	vanityData := map[string]interface{}{
		"code": find,
	}
	vanityBody, _ := json.Marshal(vanityData)

	headers := map[string]string{
		"X-Discord-MFA-Authorization": mfaToken,
		"X-Context-Properties":        "eyJsb2NhdGlvbiI6IlNlcnZlciBTZXR0aW5ncyJ9",
		"Origin":                      "https://discord.com",
		"Accept":                      "*/*",
		"Accept-Language":             "en-US,en;q=0.9",
		"Accept-Encoding":             "gzip, deflate, br",
		"Referer":                     "https://discord.com/channels/@me",
		"X-Debug-Options":             "bugReporterEnabled",
		"Cache-Control":               "no-cache",
		"Pragma":                      "no-cache",
		"DNT":                         "1",
		"Sec-Fetch-Dest":              "empty",
		"Sec-Fetch-Mode":              "cors",
		"Sec-Fetch-Site":              "same-origin",
		"TE":                          "trailers",
	}

	vanityResp, err := makeRequest("PATCH", fmt.Sprintf("/api/v10/guilds/%s/vanity-url", config.GuildID), headers, vanityBody)
	if err != nil {
		return
	}

	response := gjson.ParseBytes(vanityResp)
	code := response.Get("code").Int()

	if code == 200 {
		fmt.Println("SUCCESS:", find)
		notifyWebhook(find, string(vanityResp))
	} else if code == 60003 {
		refreshMfaToken()
		vanityUpdate(find)
	} else {
		notifyWebhook(find, string(vanityResp))
	}
}

// made by @y_ga

func notifyWebhook(find, response string) {
	pinger, _ := base64.StdEncoding.DecodeString("QGV2ZXJ5b25l")

	webhook := map[string]interface{}{
		"content":   fmt.Sprintf("%s **%s**", string(pinger), find),
		"username":  "Valhalla",
		"avatar_url": "https://files.catbox.moe/dg9s5i.jpg",
		"embeds": []map[string]interface{}{
			{
				"title":       "claimer",
				"description": fmt.Sprintf("```%s```", response),
				"color":       0x000000,
				"thumbnail": map[string]string{
					"url": "https://files.catbox.moe/opa3zq.jpg",
				},
				"fields": []map[string]interface{}{
					{
						"name":   "URL",
						"value":  fmt.Sprintf("`%s`", find),
						"inline": true,
					},
				},
				"footer": map[string]string{
					"text":    "",
					"icon_url": "https://files.catbox.moe/3civli.jpg",
				},
			},
		},
	}

	webhookBody, _ := json.Marshal(webhook)
	req, _ := http.NewRequest("POST", config.Webhook, bytes.NewBuffer(webhookBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	client.Do(req)
}

// made by @y_ga

func connectWebSocket() {
	for {
		ctx, cancel := context.WithCancel(context.Background())

		header := http.Header{}
		header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0")
		header.Set("Origin", "https://canary.discord.com")

		c, _, err := websocket.DefaultDialer.Dial("wss://gateway.discord.gg", header)
		if err != nil {
			cancel()
			time.Sleep(5 * time.Second)
			continue
		}

		var lastSequence *int64

		go func() {
			defer c.Close()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, message, err := c.ReadMessage()
					if err != nil {
						cancel()
						return
					}

					payload := gjson.ParseBytes(message)
					op := payload.Get("op").Int()

					if seq := payload.Get("s"); seq.Exists() && seq.Type != gjson.Null {
						seqNum := seq.Int()
						lastSequence = &seqNum
					}

					switch op {
					case 10:
						identify := map[string]interface{}{
							"op": 2,
							"d": map[string]interface{}{
								"token":   config.DiscordToken,
								"intents": 1,
								"properties": map[string]string{
									"os":      "Linux",
									"browser": "Firefox",
									"device":  "Firefox",
								},
							},
						}
						identifyData, _ := json.Marshal(identify)
						c.WriteMessage(websocket.TextMessage, identifyData)

						interval := payload.Get("d.heartbeat_interval").Int()
						go func() {
							ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
							defer ticker.Stop()
							for {
								select {
								case <-ctx.Done():
									return
								case <-ticker.C:
									heartbeat := map[string]interface{}{
										"op": 1,
										"d":  lastSequence,
									}
									heartbeatData, _ := json.Marshal(heartbeat)
									if err := c.WriteMessage(websocket.TextMessage, heartbeatData); err != nil {
										cancel()
										return
									}
								}
							}
						}()

					case 0:
						eventType := payload.Get("t").String()

						if eventType == "GUILD_UPDATE" {
							guildID := payload.Get("d.guild_id").String()
							vanityCode := payload.Get("d.vanity_url_code").String()

							mu.Lock()
							find, exists := guilds[guildID]
							mu.Unlock()

							if exists && find != vanityCode {
								go vanityUpdate(find)
							}
						} else if eventType == "READY" {
							guildsData := payload.Get("d.guilds").Array()

							mu.Lock()
							for _, guild := range guildsData {
								if code := guild.Get("vanity_url_code"); code.Exists() && code.Type != gjson.Null {
									guilds[guild.Get("id").String()] = code.String()
								}
							}
							mu.Unlock()
						}

					case 7:
						cancel()
						return
					}
				}
			}
		}()

		<-ctx.Done()
		time.Sleep(5 * time.Second)
	}
}
