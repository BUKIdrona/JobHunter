package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// User is the candidate this hunter applies for.
type User struct {
	Name     string `yaml:"name"`
	Email    string `yaml:"email"`
	Phone    string `yaml:"phone"`
	LinkedIn string `yaml:"linkedin"`
	GitHub   string `yaml:"github"`
	LeetCode string `yaml:"leetcode"`
	Location string `yaml:"location"`
	Resume   string `yaml:"resume"`
}

// SMTP is the mailbox used to send applications and digests.
type SMTP struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"-"`
	Password string `yaml:"-"`
	From     string `yaml:"-"`
}

// Config is the full runtime configuration.
type Config struct {
	Interval         time.Duration
	Lookback         time.Duration
	MinScore         int
	MaxHRPerRun      int
	MaxHRPerDay      int
	MaxDigest        int
	MaxDetailFetches int
	AcceptAnyRemote  bool
	DryRun           bool
	StatePath        string
	OutboxDir        string
	LockPath         string
	User             User
	SMTP             SMTP
	Greenhouse       []string
	Lever            []string
	Ashby            []string
	RemotiveQueries  []string
	JobicyTags       []string
	Indeed           []IndeedQuery
	Naukri           []NaukriQuery
}

// IndeedQuery is one public Indeed search.
type IndeedQuery struct {
	Query    string `yaml:"query"`
	Location string `yaml:"location"`
}

// NaukriQuery is one public Naukri search.
type NaukriQuery struct {
	Keyword    string `yaml:"keyword"`
	Experience int    `yaml:"experience"`
}

type fileConfig struct {
	Interval         string        `yaml:"interval"`
	Lookback         string        `yaml:"lookback"`
	MinScore         int           `yaml:"min_score"`
	MaxHRPerRun      int           `yaml:"max_hr_emails_per_run"`
	MaxHRPerDay      int           `yaml:"max_hr_emails_per_day"`
	MaxDigest        int           `yaml:"max_digest_jobs"`
	MaxDetailFetches int           `yaml:"max_detail_fetches"`
	AcceptAnyRemote  bool          `yaml:"accept_any_remote"`
	StatePath        string        `yaml:"state_path"`
	OutboxDir        string        `yaml:"outbox_dir"`
	User             User          `yaml:"user"`
	SMTP             SMTP          `yaml:"smtp"`
	Greenhouse       []string      `yaml:"greenhouse"`
	Lever            []string      `yaml:"lever"`
	Ashby            []string      `yaml:"ashby"`
	RemotiveQueries  []string      `yaml:"remotive_queries"`
	JobicyTags       []string      `yaml:"jobicy_tags"`
	Indeed           []IndeedQuery `yaml:"indeed"`
	Naukri           []NaukriQuery `yaml:"naukri"`
}

// Load reads config.yaml, then applies environment variables and .env.
func Load(path string) (*Config, error) {
	loadDotEnv(".env")

	cfg := defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var file fileConfig
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.apply(file); err != nil {
		return nil, err
	}
	cfg.applyEnv()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Interval:         2 * time.Hour,
		Lookback:         168 * time.Hour,
		MinScore:         10,
		MaxHRPerRun:      8,
		MaxHRPerDay:      20,
		MaxDigest:        25,
		MaxDetailFetches: 80,
		StatePath:        "data/state.json",
		OutboxDir:        "data/outbox",
		LockPath:         "data/jobhunter.lock",
		SMTP: SMTP{
			Host: "smtp.gmail.com",
			Port: 587,
		},
		RemotiveQueries: []string{"golang", "backend", "kubernetes", "c++"},
		JobicyTags:      []string{"golang", "backend"},
		Indeed: []IndeedQuery{
			{Query: "golang developer", Location: "India"},
			{Query: "golang developer", Location: "Gurugram"},
			{Query: "c++ developer", Location: "India"},
		},
		Naukri: []NaukriQuery{
			{Keyword: "golang developer", Experience: 1},
			{Keyword: "backend developer", Experience: 1},
			{Keyword: "c++ developer", Experience: 1},
		},
	}
}

func (c *Config) apply(file fileConfig) error {
	if file.Interval != "" {
		d, err := time.ParseDuration(file.Interval)
		if err != nil {
			return fmt.Errorf("interval: %w", err)
		}
		c.Interval = d
	}
	if file.Lookback != "" {
		d, err := time.ParseDuration(file.Lookback)
		if err != nil {
			return fmt.Errorf("lookback: %w", err)
		}
		c.Lookback = d
	}
	if file.MinScore > 0 {
		c.MinScore = file.MinScore
	}
	if file.MaxHRPerRun > 0 {
		c.MaxHRPerRun = file.MaxHRPerRun
	}
	if file.MaxHRPerDay > 0 {
		c.MaxHRPerDay = file.MaxHRPerDay
	}
	if file.MaxDigest > 0 {
		c.MaxDigest = file.MaxDigest
	}
	if file.MaxDetailFetches > 0 {
		c.MaxDetailFetches = file.MaxDetailFetches
	}
	c.AcceptAnyRemote = file.AcceptAnyRemote
	if file.StatePath != "" {
		c.StatePath = file.StatePath
	}
	if file.OutboxDir != "" {
		c.OutboxDir = file.OutboxDir
	}
	if file.User.Name != "" {
		c.User = file.User
	}
	if file.SMTP.Host != "" {
		c.SMTP.Host = file.SMTP.Host
	}
	if file.SMTP.Port != 0 {
		c.SMTP.Port = file.SMTP.Port
	}
	if file.Greenhouse != nil {
		c.Greenhouse = file.Greenhouse
	}
	if file.Lever != nil {
		c.Lever = file.Lever
	}
	if file.Ashby != nil {
		c.Ashby = file.Ashby
	}
	if file.RemotiveQueries != nil {
		c.RemotiveQueries = file.RemotiveQueries
	}
	if file.JobicyTags != nil {
		c.JobicyTags = file.JobicyTags
	}
	if file.Indeed != nil {
		c.Indeed = file.Indeed
	}
	if file.Naukri != nil {
		c.Naukri = file.Naukri
	}
	return nil
}

func (c *Config) applyEnv() {
	if v := os.Getenv("SMTP_HOST"); v != "" {
		c.SMTP.Host = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.SMTP.Port = n
		}
	}
	c.SMTP.Username = firstNonEmpty(os.Getenv("SMTP_USERNAME"), c.User.Email)
	c.SMTP.Password = os.Getenv("SMTP_PASSWORD")
	c.SMTP.From = firstNonEmpty(os.Getenv("SMTP_FROM"), c.SMTP.Username, c.User.Email)
	if v := os.Getenv("USER_EMAIL"); v != "" {
		c.User.Email = v
	}
	if v := strings.TrimSpace(os.Getenv("DRY_RUN")); v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		c.DryRun = true
	}
}

func (c *Config) validate() error {
	if c.User.Email == "" || !strings.Contains(c.User.Email, "@") {
		return fmt.Errorf("user.email is missing")
	}
	if c.User.Name == "" {
		return fmt.Errorf("user.name is missing")
	}
	if c.Interval < time.Minute {
		return fmt.Errorf("interval must be at least 1 minute")
	}
	if strings.TrimSpace(c.SMTP.Password) == "" {
		c.DryRun = true
	}
	return nil
}

// LoadDotEnv is used by tests and startup. Blank lines and comments are ignored.
// Existing environment variables win.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
