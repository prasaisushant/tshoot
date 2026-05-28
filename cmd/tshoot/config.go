package main

// Config holds all application configuration
type Config struct {
	General      GeneralConfig      `toml:"general"`
	RefreshRates RefreshRatesConfig `toml:"refresh_rates"`
	Alerts       AlertsConfig       `toml:"alerts"`
	Docker       DockerConfig       `toml:"docker"`
}

// GeneralConfig holds general application settings
type GeneralConfig struct {
	RefreshRateDefault int    `toml:"refresh_rate_default"`
	Theme              string `toml:"theme"`
	StartupMode        string `toml:"startup_mode"`
}

// RefreshRatesConfig holds per-panel refresh rates (in seconds)
type RefreshRatesConfig struct {
	CPU     int `toml:"cpu"`
	Memory  int `toml:"memory"`
	Ports   int `toml:"ports"`
	Docker  int `toml:"docker"`
	Ping    int `toml:"ping"`
	Storage int `toml:"storage"`
}

// AlertsConfig holds alert threshold settings
type AlertsConfig struct {
	CPUWarn  int `toml:"cpu_warn"`
	CPUCrit  int `toml:"cpu_crit"`
	MemWarn  int `toml:"mem_warn"`
	DiskWarn int `toml:"disk_warn"`
}

// DockerConfig holds Docker-specific settings
type DockerConfig struct {
	Socket      string `toml:"socket"`
	DefaultTail int    `toml:"default_tail"`
	Follow      bool   `toml:"follow"`
}

// LoadConfig loads the configuration from file, or returns hardcoded defaults.
// TODO: In Phase 7, implement actual file loading from ~/.config/tshoot/config.toml
func LoadConfig() *Config {
	return &Config{
		General: GeneralConfig{
			RefreshRateDefault: 3,
			Theme:              "dark",
			StartupMode:        "dashboard",
		},
		RefreshRates: RefreshRatesConfig{
			CPU:     1,
			Memory:  1,
			Ports:   5,
			Docker:  3,
			Ping:    10,
			Storage: 15,
		},
		Alerts: AlertsConfig{
			CPUWarn:  80,
			CPUCrit:  95,
			MemWarn:  80,
			DiskWarn: 90,
		},
		Docker: DockerConfig{
			Socket:      "/var/run/docker.sock",
			DefaultTail: 100,
			Follow:      true,
		},
	}
}
