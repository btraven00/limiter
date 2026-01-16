package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
)

const (
	delegateConfigDir     = "/etc/systemd/system/user@.service.d"
	delegateConfigFile    = "delegate.conf"
	limitsConfigPath      = "/etc/limiter/limits.cfg"
	delegateConfigContent = `[Service]
Delegate=cpu cpuset memory pids
`
)

type Config struct {
	Ram     string
	Cores   float64
	Procs   int
	Timeout string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "setup":
		runSetup()
	case "verify":
		runVerify()
	case "run", "--":
		runCommand(os.Args[2:])
	default:
		// Default behavior: treat as run command
		runCommand(os.Args[1:])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `limiter - Resource-limited command execution

Usage:
  limiter setup                          Setup systemd delegation (requires root)
  limiter verify                         Verify systemd delegation is configured
  limiter [flags] -- <command> [args]    Run command with resource limits

Flags:
  -ram <size>       Memory limit (e.g., 2G, 500M)
  -cores <number>   CPU cores limit (e.g., 1.5)
  -procs <number>   Max number of processes/threads
  -timeout <dur>    Execution timeout (e.g., 1h, 30m, 10s, 7d)

Defaults (if not specified):
  Config file: %s
  Fallback: ram=1G, cores=20%% of system, procs=100, timeout=7d

Examples:
  limiter setup
  limiter verify
  limiter -ram 2G -cores 1.5 -timeout 1h -- ./my-program
  limiter -- ./my-program  # Uses defaults
`, limitsConfigPath)
}

func runSetup() {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: setup requires root privileges\n")
		fmt.Fprintf(os.Stderr, "Usage: sudo limiter setup\n")
		os.Exit(1)
	}

	fmt.Println("Setting up systemd resource delegation...")

	// Create directory for systemd delegation
	if err := os.MkdirAll(delegateConfigDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", delegateConfigDir, err)
		os.Exit(1)
	}

	// Write delegation configuration file
	configPath := filepath.Join(delegateConfigDir, delegateConfigFile)
	if err := os.WriteFile(configPath, []byte(delegateConfigContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config file %s: %v\n", configPath, err)
		os.Exit(1)
	}

	fmt.Printf("Created: %s\n", configPath)

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reloading systemd: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Systemd delegation configured successfully")

	// Optionally create default limits config
	limitsDir := filepath.Dir(limitsConfigPath)
	if err := os.MkdirAll(limitsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create %s: %v\n", limitsDir, err)
	} else {
		if _, err := os.Stat(limitsConfigPath); os.IsNotExist(err) {
			defaultCores := float64(runtime.NumCPU()) * 0.2
			defaultConfig := fmt.Sprintf("# Default resource limits\nram=1G\ncores=%.1f\nprocs=100\ntimeout=7d\n", defaultCores)
			if err := os.WriteFile(limitsConfigPath, []byte(defaultConfig), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create default config %s: %v\n", limitsConfigPath, err)
			} else {
				fmt.Printf("Created default config: %s\n", limitsConfigPath)
			}
		}
	}

	fmt.Println()
	fmt.Println("Users can now run 'limiter' without sudo for resource control.")
}

func runVerify() {
	configPath := filepath.Join(delegateConfigDir, delegateConfigFile)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "✗ Delegation not configured\n")
		fmt.Fprintf(os.Stderr, "  Missing: %s\n", configPath)
		fmt.Fprintf(os.Stderr, "\nRun: sudo limiter setup\n")
		os.Exit(1)
	}

	// Read and verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Cannot read config file: %v\n", err)
		os.Exit(1)
	}

	// Basic verification that delegation is set
	contentStr := string(content)
	if !strings.Contains(contentStr, "Delegate=") {
		fmt.Fprintf(os.Stderr, "✗ Config file exists but delegation not properly set\n")
		fmt.Fprintf(os.Stderr, "  Expected 'Delegate=' directive in %s\n", configPath)
		os.Exit(1)
	}

	// Check for expected resources
	hasAll := strings.Contains(contentStr, "cpu") &&
		strings.Contains(contentStr, "memory") &&
		strings.Contains(contentStr, "pids")

	if !hasAll {
		fmt.Fprintf(os.Stderr, "⚠ Warning: Delegation may be incomplete\n")
		fmt.Fprintf(os.Stderr, "  Expected: cpu, memory, pids\n")
		fmt.Fprintf(os.Stderr, "  Current content:\n%s\n", contentStr)
	}

	fmt.Println("✓ Systemd delegation is configured")
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Println()
	fmt.Println("You can run limiter commands without sudo.")
}

func loadConfig() Config {
	cfg := Config{
		Ram:     "1G",
		Cores:   float64(runtime.NumCPU()) * 0.2,
		Procs:   100,
		Timeout: "168h", // 7 days in hours
	}

	file, err := os.Open(limitsConfigPath)
	if err != nil {
		return cfg // Use defaults if config doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "ram":
			cfg.Ram = value
		case "cores":
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				cfg.Cores = f
			}
		case "procs":
			if i, err := strconv.Atoi(value); err == nil {
				cfg.Procs = i
			}
		case "timeout":
			cfg.Timeout = value
		}
	}

	return cfg
}

// parseDuration extends time.ParseDuration to support days (d)
func parseDuration(s string) (time.Duration, error) {
	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		dayStr := strings.TrimSuffix(s, "d")
		days, err := strconv.ParseFloat(dayStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day format: %v", err)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func runCommand(args []string) {
	// Load defaults from config
	defaults := loadConfig()

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	memStr := fs.String("ram", "", "Memory limit (e.g., 2G, 500M)")
	cpuCores := fs.Float64("cores", 0, "CPU cores limit (e.g., 1.5)")
	pidsLimit := fs.Int("procs", 0, "Max number of processes/threads")
	timeoutStr := fs.String("timeout", "", "Execution timeout (e.g., 1h, 30m, 10s, 7d)")

	fs.Usage = func() {
		printUsage()
	}

	// Find the separator "--"
	var flagArgs, cmdArgs []string
	foundSep := false
	for i, arg := range args {
		if arg == "--" {
			flagArgs = args[:i]
			cmdArgs = args[i+1:]
			foundSep = true
			break
		}
	}

	if !foundSep {
		flagArgs = args
		cmdArgs = []string{}
	}

	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if !foundSep {
		cmdArgs = fs.Args()
	}

	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n\n")
		printUsage()
		os.Exit(1)
	}

	// Apply defaults for unspecified flags
	if *memStr == "" {
		*memStr = defaults.Ram
	}
	if *cpuCores == 0 {
		*cpuCores = defaults.Cores
	}
	if *pidsLimit == 0 {
		*pidsLimit = defaults.Procs
	}
	if *timeoutStr == "" {
		*timeoutStr = defaults.Timeout
	}

	// Build the systemd-run command
	sysArgs := []string{"--user", "--scope"}

	var appliedLimits []string

	// Handle Memory
	if *memStr != "" {
		limit, err := units.RAMInBytes(*memStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid memory format: %v\n", err)
			os.Exit(1)
		}
		sysArgs = append(sysArgs, "-p", fmt.Sprintf("MemoryMax=%d", limit))
		appliedLimits = append(appliedLimits, fmt.Sprintf("ram=%s", *memStr))
	}

	// Handle CPU
	if *cpuCores > 0 {
		quota := int(*cpuCores * 100)
		sysArgs = append(sysArgs, "-p", fmt.Sprintf("CPUQuota=%d%%", quota))
		appliedLimits = append(appliedLimits, fmt.Sprintf("cores=%.1f", *cpuCores))
	}

	// Handle PIDs
	if *pidsLimit > 0 {
		sysArgs = append(sysArgs, "-p", fmt.Sprintf("TasksMax=%d", *pidsLimit))
		appliedLimits = append(appliedLimits, fmt.Sprintf("procs=%d", *pidsLimit))
	}

	// Handle Timeout
	if *timeoutStr != "" {
		duration, err := parseDuration(*timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid timeout format (use 1h, 10m, 10s, 7d): %v\n", err)
			os.Exit(1)
		}
		sysArgs = append(sysArgs, "-p", fmt.Sprintf("RuntimeMaxSec=%v", duration.Seconds()))
		appliedLimits = append(appliedLimits, fmt.Sprintf("timeout=%s", *timeoutStr))
	}

	// Append the actual command
	sysArgs = append(sysArgs, cmdArgs...)

	// Execute
	fmt.Printf("Limits: %s\n", strings.Join(appliedLimits, ", "))
	fmt.Printf("Command: %s\n", strings.Join(cmdArgs, " "))
	cmd := exec.Command("systemd-run", sysArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
		os.Exit(1)
	}
}
