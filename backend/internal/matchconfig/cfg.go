package matchconfig

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	cfgBaseDir     = "configs"
	serverInitPath = "configs/server_init.cfg"
)

// GetCFG returns the content of a phase CFG for a profile.
// Returns empty string (not error) if the file doesn't exist yet.
func GetCFG(profileID, phase string) (string, error) {
	data, err := os.ReadFile(filepath.Join(cfgBaseDir, profileID, phase+".cfg"))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// SetCFG writes the content of a phase CFG for a profile.
func SetCFG(profileID, phase, content string) error {
	dir := filepath.Join(cfgBaseDir, profileID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, phase+".cfg"), []byte(content), 0644)
}

// DeleteProfileCFGs removes all CFG files for a profile.
func DeleteProfileCFGs(profileID string) error {
	return os.RemoveAll(filepath.Join(cfgBaseDir, profileID))
}

// GetServerInitCFG returns the server init config content.
func GetServerInitCFG() (string, error) {
	data, err := os.ReadFile(serverInitPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// SetServerInitCFG writes the server init config content.
func SetServerInitCFG(content string) error {
	if err := os.MkdirAll(cfgBaseDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(serverInitPath, []byte(content), 0644)
}

// GetServerInitCommands returns parsed RCON commands from server_init.cfg.
func GetServerInitCommands() []string {
	content, _ := GetServerInitCFG()
	return ParseCFG(content)
}

// GetProfileWarmupCommands returns parsed RCON commands from a profile's warmup CFG.
func GetProfileWarmupCommands(profileID string) []string {
	content, _ := GetCFG(profileID, "warmup")
	return ParseCFG(content)
}

// GetProfilePhaseCommands returns parsed RCON commands from a profile's CFG for any phase.
func GetProfilePhaseCommands(profileID, phase string) []string {
	content, _ := GetCFG(profileID, phase)
	return ParseCFG(content)
}

// ParseCFG returns executable RCON commands from CFG content.
// Skips empty lines and comment-only lines (// ...).
func ParseCFG(content string) []string {
	var cmds []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		// Strip trailing inline comments (space + //)
		if idx := strings.Index(line, " //"); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			cmds = append(cmds, line)
		}
	}
	return cmds
}

// seedServerInitCFG creates the default server init cfg if it doesn't exist.
func seedServerInitCFG() {
	if _, err := os.Stat(serverInitPath); err == nil {
		return
	}
	_ = os.MkdirAll(cfgBaseDir, 0755)
	_ = os.WriteFile(serverInitPath, []byte(defaultServerInitCFG), 0644)
}

// ─── Default CFG content ───────────────────────────────────────────────────

const defaultServerInitCFG = `hostname "Warmup deathmatch"
// Mode deathmatch par défaut (annulé au lancement d'un match)
game_type 1
game_mode 2
mp_respawn_on_death_ct 1
mp_respawn_on_death_t 1
mp_free_armor 1
mp_ignore_round_win_conditions 1
mp_autokick 0
sv_hibernate_when_empty 0
sv_clockcorrection_msecs 15
// GOTV
tv_enable 1
tv_delay 0
tv_autorecord 0
tv_allow_camera_man 0
tv_allow_static_shots 1
tv_chatgroupsize 0
tv_debug 0
tv_delaymapchange 1
tv_deltacache 2
tv_dispatchmode 1
tv_maxclients 10
tv_maxrate 0
tv_relayvoice 0
tv_timeout 60
tv_transmitall 1
tv_advertise_watchable 1
`

const defaultWarmupCFG = `game_type 0
game_mode 1
bot_kick
bot_quota 0
mp_autokick 0
mp_autoteambalance 0
mp_buy_anywhere 0
mp_buytime 15
mp_death_drop_gun 0
mp_free_armor 0
mp_ignore_round_win_conditions 0
mp_limitteams 0
mp_respawn_on_death_ct 0
mp_respawn_on_death_t 0
mp_solid_teammates 0
mp_spectators_max 20
mp_maxmoney 16000
mp_startmoney 16000
mp_timelimit 0
sv_alltalk 0
sv_auto_full_alltalk_during_warmup_half_end 0
sv_deadtalk 1
sv_full_alltalk 0
sv_hibernate_when_empty 0
mp_weapons_allow_typecount -1
sv_infinite_ammo 0
sv_showimpacts 0
sv_voiceenable 1
tv_relayvoice 1
sv_cheats 0
mp_ct_default_melee weapon_knife
mp_ct_default_secondary weapon_hkp2000
mp_ct_default_primary ""
mp_t_default_melee weapon_knife
mp_t_default_secondary weapon_glock
mp_t_default_primary ""
mp_maxrounds 24
mp_warmuptime 9999
mp_warmup_pausetimer 1
cash_team_bonus_shorthanded 0
sv_human_autojoin_team 0
`

const defaultKnifeCFG = `mp_ct_default_secondary ""
mp_free_armor 1
mp_freezetime 10
mp_give_player_c4 0
mp_maxmoney 0
mp_respawn_immunitytime 0
mp_respawn_on_death_ct 0
mp_respawn_on_death_t 0
mp_roundtime 1.92
mp_roundtime_defuse 1.92
mp_roundtime_hostage 1.92
mp_t_default_secondary ""
mp_round_restart_delay 3
mp_team_intro_time 0
mp_restartgame 1
mp_warmup_end
mp_solid_teammates 1
`

const defaultLiveCFG = `ammo_grenade_limit_default 1
ammo_grenade_limit_flashbang 2
ammo_grenade_limit_total 4
bot_quota 0
cash_player_bomb_defused 300
cash_player_bomb_planted 300
cash_player_damage_hostage -30
cash_player_interact_with_hostage 300
cash_player_killed_enemy_default 300
cash_player_killed_enemy_factor 1
cash_player_killed_hostage -1000
cash_player_killed_teammate -300
cash_player_rescued_hostage 1000
cash_team_elimination_bomb_map 3250
cash_team_elimination_hostage_map_ct 3000
cash_team_elimination_hostage_map_t 3000
cash_team_hostage_alive 0
cash_team_hostage_interaction 600
cash_team_loser_bonus 1400
cash_team_loser_bonus_consecutive_rounds 500
cash_team_planted_bomb_but_defused 600
cash_team_rescued_hostage 600
cash_team_terrorist_win_bomb 3500
cash_team_win_by_defusing_bomb 3500
cash_team_win_by_hostage_rescue 2900
cash_team_win_by_time_running_out_bomb 3250
cash_team_win_by_time_running_out_hostage 3250
ff_damage_reduction_bullets 0.33
ff_damage_reduction_grenade 0.85
ff_damage_reduction_grenade_self 1
ff_damage_reduction_other 0.4
mp_afterroundmoney 0
mp_autokick 0
mp_autoteambalance 0
mp_backup_restore_load_autopause 1
mp_backup_round_auto 1
mp_buy_anywhere 0
mp_buy_during_immunity 0
mp_buytime 20
mp_c4timer 40
mp_ct_default_melee weapon_knife
mp_ct_default_primary ""
mp_ct_default_secondary weapon_hkp2000
mp_death_drop_defuser 1
mp_death_drop_grenade 2
mp_death_drop_gun 1
mp_defuser_allocation 0
mp_display_kill_assists 1
mp_endmatch_votenextmap 0
mp_forcecamera 1
mp_free_armor 0
mp_freezetime 18
mp_friendlyfire 1
mp_give_player_c4 1
mp_halftime 1
mp_halftime_duration 15
mp_ignore_round_win_conditions 0
mp_limitteams 0
mp_match_can_clinch 1
mp_match_end_restart 0
mp_maxmoney 16000
mp_playercashawards 1
mp_randomspawn 0
mp_respawn_immunitytime 0
mp_respawn_on_death_ct 0
mp_respawn_on_death_t 0
mp_round_restart_delay 5
mp_roundtime 1.92
mp_roundtime_defuse 1.92
mp_roundtime_hostage 1.92
mp_solid_teammates 1
mp_starting_losses 1
mp_startmoney 800
mp_t_default_melee weapon_knife
mp_t_default_primary ""
mp_t_default_secondary weapon_glock
mp_teamcashawards 1
mp_timelimit 0
mp_weapons_allow_map_placed 1
mp_weapons_allow_zeus 1
mp_win_panel_display_time 3
spec_freeze_deathanim_time 0
spec_freeze_time 2
spec_freeze_time_lock 2
spec_replay_enable 0
sv_allow_votes 1
sv_auto_full_alltalk_during_warmup_half_end 0
sv_damage_print_enable 0
sv_deadtalk 1
sv_hibernate_postgame_delay 300
sv_ignoregrenaderadio 0
sv_infinite_ammo 0
sv_talk_enemy_dead 0
sv_talk_enemy_living 0
sv_voiceenable 1
tv_relayvoice 1
sv_vote_command_delay 0
cash_team_bonus_shorthanded 0
mp_spectators_max 20
mp_team_intro_time 0
mp_disconnect_kills_players 0
mp_warmup_end
`
