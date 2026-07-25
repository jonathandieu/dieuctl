package cmd

import (
	"fmt"
	"os"

	"github.com/jonathandieu/dieuctl/cmd/argocd"
	"github.com/jonathandieu/dieuctl/cmd/auth"
	"github.com/jonathandieu/dieuctl/cmd/cluster"
	"github.com/jonathandieu/dieuctl/cmd/exec"
	"github.com/jonathandieu/dieuctl/cmd/machine"
	"github.com/jonathandieu/dieuctl/cmd/traffic"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "dieuctl",
	Short: "A personal swiss-army knife CLI",
	Long:  "A personal swiss-army knife CLI for automating the things I'm too lazy to do manually but too stubborn to do differently.",
}

func init() {
	rootCmd.AddCommand(cluster.Cmd, argocd.Cmd, auth.Cmd, exec.Cmd, machine.Cmd, traffic.Cmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/dieuctl/config.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		viper.AddConfigPath(home + "/.config/dieuctl")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.ReadInConfig()
}
