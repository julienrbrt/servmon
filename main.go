package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
)

var (
	version    = "1.0.0"
	flagConfig = "config"
	cfgFile    string
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user home directory: %v", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:     "servmon",
		Short:   "Server Monitoring Tool with TUI and Alerts",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			go monitorHTTP(cfg)

			select {} // keep alive
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, flagConfig, path.Join(homeDir, ".servmon.yaml"), "config file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}
