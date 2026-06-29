package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv            string
	LogLevel          string
	HTTPListen        string
	WSPath            string
	EventQueue        int
	SlowConsumerQueue int

	EnableTZSPUDP bool
	TZSPListen    string

	EnableRawSniff bool
	SniffInterface string
	SniffFilter    string
	SniffPromisc   bool

	DictionaryGlob string

	EnableMDNS bool
	MDNSName   string
	MDNSPort   int

	DBPath            string
	ForwardAuthTarget string
	ForwardAcctTarget string
}

func Load() Config {
	return Config{
		AppEnv:            env("URSA_APP_ENV", "development"),
		LogLevel:          env("URSA_LOG_LEVEL", ""),
		HTTPListen:        env("URSA_TZSP_HTTP_LISTEN", ":8098"),
		WSPath:            env("URSA_TZSP_WS_PATH", "/ws"),
		EventQueue:        envInt("URSA_TZSP_EVENT_QUEUE", 1024),
		SlowConsumerQueue: envInt("URSA_TZSP_CLIENT_QUEUE", 128),

		EnableTZSPUDP: envBool("URSA_TZSP_ENABLE_UDP", true),
		TZSPListen:    env("URSA_TZSP_UDP_LISTEN", ":37008"),

		EnableRawSniff: envBool("URSA_TZSP_ENABLE_RAW_SNIFF", false),
		SniffInterface: env("URSA_TZSP_SNIFF_IFACE", "eth0"),
		SniffFilter:    env("URSA_TZSP_SNIFF_FILTER", "udp and (port 1812 or port 1813)"),
		SniffPromisc:   envBool("URSA_TZSP_SNIFF_PROMISCUOUS", false),

		DictionaryGlob: env("URSA_TZSP_DICTIONARY_GLOB", "./config/dictionary/dictionary.*"),

		EnableMDNS: envBool("URSA_TZSP_ENABLE_MDNS", true),
		MDNSName:   env("URSA_TZSP_MDNS_NAME", "tzsp-radius-collector"),
		MDNSPort:   envInt("URSA_TZSP_MDNS_PORT", 8098),

		DBPath:            env("URSA_TZSP_DB_PATH", "./data/collector.db"),
		ForwardAuthTarget: env("URSA_TZSP_FORWARD_AUTH_TARGET", "127.0.0.1:1812"),
		ForwardAcctTarget: env("URSA_TZSP_FORWARD_ACCT_TARGET", "127.0.0.1:1813"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
