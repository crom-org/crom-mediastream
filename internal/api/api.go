package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type TwitchAPI struct {
	ClientID     string
	AccessToken  string
	BroadcasterID string
}

func NewTwitchAPI(clientID, accessToken, broadcasterID string) *TwitchAPI {
	return &TwitchAPI{
		ClientID:      clientID,
		AccessToken:   accessToken,
		BroadcasterID: broadcasterID,
	}
}

// UpdateStreamMetadata updates the title on Twitch
func (t *TwitchAPI) UpdateStreamMetadata(title string) error {
	if t.ClientID == "" || t.AccessToken == "" || t.BroadcasterID == "" {
		return nil // Skip if not configured
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/channels?broadcaster_id=%s", t.BroadcasterID)
	
	body := map[string]interface{}{
		"title":   title,
		"game_id": "1469308723", // Categoria: Software and Game Development
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Client-Id", t.ClientID)
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("twitch api error: status %d", resp.StatusCode)
	}

	return nil
}

type YouTubeAPI struct {
	APIKey string
}

func (y *YouTubeAPI) UpdateTitle(title string) error {
	// YouTube API update requires OAuth and is much more complex.
	// For Phase 4 "Simple", we'll provide a placeholder or basic implementation.
	fmt.Printf("[YouTube API] Mock update title to: %s\n", title)
	return nil
}
