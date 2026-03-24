package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type RankInfo struct {
	PremierRating   int `json:"premier_rating"`
	CompetitiveRank int `json:"competitive_rank"`
	CompetitiveWins int `json:"competitive_wins"`
}

type Manager struct {
	port    string
	botDir  string
	cmd     *exec.Cmd
	baseURL string
}

func NewManager(port string) *Manager {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..", "bot")

	return &Manager{
		port:    port,
		botDir:  root,
		baseURL: fmt.Sprintf("http://127.0.0.1:%s", port),
	}
}

func (m *Manager) Start(ctx context.Context) {
	go m.run(ctx)
}

func (m *Manager) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Println("[bot] Starting Node.js subprocess...")
		m.cmd = exec.CommandContext(ctx, "node", "index.js")
		m.cmd.Dir = m.botDir
		m.cmd.Stdin  = os.Stdin
		m.cmd.Stdout = os.Stdout
		m.cmd.Stderr = os.Stderr
		m.cmd.Env = append(os.Environ(),
			fmt.Sprintf("BOT_PORT=%s", m.port),
		)

		if err := m.cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return // arrêt volontaire
			}
			log.Printf("[bot] Subprocess exited: %v — restarting in 5s", err)
			time.Sleep(5 * time.Second)
		}
	}
}

type BotInfo struct {
	SteamID string `json:"steamid"`
}

func (m *Manager) GetInfo() (*BotInfo, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(m.baseURL + "/info")
	if err != nil {
		return nil, fmt.Errorf("bot unreachable: %w", err)
	}
	defer resp.Body.Close()

	var info BotInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (m *Manager) GetRank(steamID string) (*RankInfo, error) {
	url := fmt.Sprintf("%s/rank/%s", m.baseURL, steamID)

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("bot unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("%s", errResp.Error)
		}
		return nil, fmt.Errorf("bot returned %d", resp.StatusCode)
	}

	var info RankInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
