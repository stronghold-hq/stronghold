package cli

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigGet retrieves a configuration value by key using dot notation
func ConfigGet(key string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if key == "" {
		// Show all config
		data, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	value, err := getConfigValue(config, key)
	if err != nil {
		return err
	}

	// Format the output
	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case bool:
		fmt.Println(v)
	case int:
		fmt.Println(v)
	case float64:
		fmt.Println(v)
	case ScanTypeConfig:
		fmt.Printf("enabled: %v\n", v.Enabled)
		fmt.Printf("action_on_warn: %s\n", v.ActionOnWarn)
		fmt.Printf("action_on_block: %s\n", v.ActionOnBlock)
	case ScanningConfig:
		fmt.Printf("mode: %s\n", v.Mode)
		fmt.Printf("block_threshold: %.2f\n", v.BlockThreshold)
		fmt.Printf("fail_open: %v\n", v.FailOpen)
		fmt.Println("content:")
		fmt.Printf("  enabled: %v\n", v.Content.Enabled)
		fmt.Printf("  action_on_warn: %s\n", v.Content.ActionOnWarn)
		fmt.Printf("  action_on_block: %s\n", v.Content.ActionOnBlock)
		fmt.Println("output:")
		fmt.Printf("  enabled: %v\n", v.Output.Enabled)
		fmt.Printf("  action_on_warn: %s\n", v.Output.ActionOnWarn)
		fmt.Printf("  action_on_block: %s\n", v.Output.ActionOnBlock)
	default:
		fmt.Printf("%v\n", v)
	}

	return nil
}

// ConfigSet sets a configuration value by key using dot notation
func ConfigSet(key, value string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := setConfigValue(config, key, value); err != nil {
		return err
	}

	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

// getConfigValue retrieves a value from the config using dot notation
func getConfigValue(config *CLIConfig, key string) (interface{}, error) {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "scanning":
		if len(parts) == 1 {
			return config.Scanning, nil
		}
		return getScanningValue(&config.Scanning, parts[1:])
	case "proxy":
		if len(parts) == 1 {
			return config.Proxy, nil
		}
		return getProxyValue(&config.Proxy, parts[1:])
	case "api":
		if len(parts) == 1 {
			return config.API, nil
		}
		return getAPIValue(&config.API, parts[1:])
	case "logging":
		if len(parts) == 1 {
			return config.Logging, nil
		}
		return getLoggingValue(&config.Logging, parts[1:])
	default:
		return nil, fmt.Errorf("unknown config key: %s", key)
	}
}

func getScanningValue(scanning *ScanningConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return *scanning, nil
	}

	switch parts[0] {
	case "mode":
		return scanning.Mode, nil
	case "block_threshold":
		return scanning.BlockThreshold, nil
	case "fail_open":
		return scanning.FailOpen, nil
	case "content":
		if len(parts) == 1 {
			return scanning.Content, nil
		}
		return getScanTypeValue(&scanning.Content, parts[1:])
	case "output":
		if len(parts) == 1 {
			return scanning.Output, nil
		}
		return getScanTypeValue(&scanning.Output, parts[1:])
	default:
		return nil, fmt.Errorf("unknown scanning key: %s", parts[0])
	}
}

func getScanTypeValue(scanType *ScanTypeConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return *scanType, nil
	}

	switch parts[0] {
	case "enabled":
		return scanType.Enabled, nil
	case "action_on_warn":
		return scanType.ActionOnWarn, nil
	case "action_on_block":
		return scanType.ActionOnBlock, nil
	default:
		return nil, fmt.Errorf("unknown scan type key: %s", parts[0])
	}
}

func getProxyValue(proxy *ProxyConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return *proxy, nil
	}

	switch parts[0] {
	case "port":
		return proxy.Port, nil
	case "bind":
		return proxy.Bind, nil
	default:
		return nil, fmt.Errorf("unknown proxy key: %s", parts[0])
	}
}

func getAPIValue(api *APIConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return *api, nil
	}

	switch parts[0] {
	case "endpoint":
		return api.Endpoint, nil
	case "timeout":
		return api.Timeout.String(), nil
	default:
		return nil, fmt.Errorf("unknown api key: %s", parts[0])
	}
}

func getLoggingValue(logging *LoggingConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return *logging, nil
	}

	switch parts[0] {
	case "level":
		return logging.Level, nil
	case "file":
		return logging.File, nil
	default:
		return nil, fmt.Errorf("unknown logging key: %s", parts[0])
	}
}

// setConfigValue sets a value in the config using dot notation
func setConfigValue(config *CLIConfig, key, value string) error {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "scanning":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire scanning section, specify a sub-key")
		}
		return setScanningValue(&config.Scanning, parts[1:], value)
	case "proxy":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire proxy section, specify a sub-key")
		}
		return setProxyValue(&config.Proxy, parts[1:], value)
	case "api":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire api section, specify a sub-key")
		}
		return setAPIValue(&config.API, parts[1:], value)
	case "logging":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire logging section, specify a sub-key")
		}
		return setLoggingValue(&config.Logging, parts[1:], value)
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}

func setScanningValue(scanning *ScanningConfig, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("missing scanning sub-key")
	}

	switch parts[0] {
	case "mode":
		if value != "smart" && value != "strict" && value != "permissive" {
			return fmt.Errorf("invalid mode: %s (must be smart, strict, or permissive)", value)
		}
		scanning.Mode = value
	case "block_threshold":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid block_threshold: %s (must be a number)", value)
		}
		if f < 0 || f > 1 {
			return fmt.Errorf("invalid block_threshold: %s (must be between 0 and 1)", value)
		}
		scanning.BlockThreshold = f
	case "fail_open":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid fail_open: %s (must be true or false)", value)
		}
		scanning.FailOpen = b
	case "content":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire content section, specify a sub-key (enabled, action_on_warn, action_on_block)")
		}
		return setScanTypeValue(&scanning.Content, parts[1:], value)
	case "output":
		if len(parts) < 2 {
			return fmt.Errorf("cannot set entire output section, specify a sub-key (enabled, action_on_warn, action_on_block)")
		}
		return setScanTypeValue(&scanning.Output, parts[1:], value)
	default:
		return fmt.Errorf("unknown scanning key: %s", parts[0])
	}

	return nil
}

func setScanTypeValue(scanType *ScanTypeConfig, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("missing scan type sub-key")
	}

	switch parts[0] {
	case "enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid enabled: %s (must be true or false)", value)
		}
		scanType.Enabled = b
	case "action_on_warn":
		if value != "allow" && value != "warn" && value != "block" {
			return fmt.Errorf("invalid action_on_warn: %s (must be allow, warn, or block)", value)
		}
		scanType.ActionOnWarn = value
	case "action_on_block":
		if value != "allow" && value != "warn" && value != "block" {
			return fmt.Errorf("invalid action_on_block: %s (must be allow, warn, or block)", value)
		}
		scanType.ActionOnBlock = value
	default:
		return fmt.Errorf("unknown scan type key: %s", parts[0])
	}

	return nil
}

func setProxyValue(proxy *ProxyConfig, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("missing proxy sub-key")
	}

	switch parts[0] {
	case "port":
		p, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid port: %s (must be a number)", value)
		}
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port: %d (must be between 1 and 65535)", p)
		}
		proxy.Port = p
	case "bind":
		proxy.Bind = value
	default:
		return fmt.Errorf("unknown proxy key: %s", parts[0])
	}

	return nil
}

func setAPIValue(api *APIConfig, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("missing api sub-key")
	}

	switch parts[0] {
	case "endpoint":
		api.Endpoint = value
	default:
		return fmt.Errorf("unknown api key: %s", parts[0])
	}

	return nil
}

func setLoggingValue(logging *LoggingConfig, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("missing logging sub-key")
	}

	switch parts[0] {
	case "level":
		if value != "debug" && value != "info" && value != "warn" && value != "error" {
			return fmt.Errorf("invalid level: %s (must be debug, info, warn, or error)", value)
		}
		logging.Level = value
	case "file":
		logging.File = value
	default:
		return fmt.Errorf("unknown logging key: %s", parts[0])
	}

	return nil
}
