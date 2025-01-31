package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagConfig  = "config"
	flagDaemon  = "daemon"
	cfgFile     string
	runAsDaemon bool
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user home directory: %v", err)
		os.Exit(1)
	}

	version, err := getVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting version: %v", err)
		version = "unknown"
	}

	rootCmd := &cobra.Command{
		Use:     "servmon",
		Short:   "KISS server monitoring tool with email alerts",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			runAsDaemon, err := cmd.Flags().GetBool(flagDaemon)
			if err != nil {
				return fmt.Errorf("error getting flag %s: %v", flagDaemon, err)
			}

			if runAsDaemon {
				pid, err := runAsDaemonProcess()
				if err != nil {
					return err
				}

				cmd.Println("Running as daemon with PID", pid)
				return nil
			}

			cfgPath, err := cmd.Flags().GetString(flagConfig)
			if err != nil {
				return fmt.Errorf("error getting flag %s: %v", flagConfig, err)
			}

			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				cfg := defaultConfig()
				if err := cfg.Save(cfgFile); err != nil {
					return err
				}

				cmd.Println("Configuration file generated at", cfgFile)
				return nil
			} else if err != nil {
				return fmt.Errorf("error checking config file: %v", err)
			}

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			go monitorCPU(cfg)
			go monitorMemory(cfg)
			go monitorDisk(cfg)
			go monitorHTTP(cfg)

			select {} // keep alive
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, flagConfig, path.Join(homeDir, ".servmon.yaml"), "config file")
	rootCmd.PersistentFlags().BoolVarP(&runAsDaemon, flagDaemon, "d", false, "run as daemon")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func runAsDaemonProcess() (int, error) {
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
		var args []string
		for _, a := range os.Args[1:] {
			if a != "-d" && a != "--daemon" {
				args = append(args, a)
			}
		}

		cmd := exec.Command(os.Args[0], args...)
		cmd.Stdout = os.NewFile(3, "log.out")
		cmd.Stderr = os.NewFile(4, "log.err")
		cmd.Stdin = os.NewFile(3, "log.in")

		if err := cmd.Start(); err != nil {
			return 0, fmt.Errorf("error starting as daemon: %v", err)
		}

		pid := cmd.Process.Pid

		// Detach the process
		err := cmd.Process.Release()
		if err != nil {
			return 0, fmt.Errorf("error detaching process: %v", err)
		}

		return pid, nil
	}

	return 0, fmt.Errorf("daemon mode is not supported on %s", runtime.GOOS)
}

func getVersion() (string, error) {
	version, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("failed to get version")
	}

	return strings.TrimSpace(version.Main.Version), nil
}
