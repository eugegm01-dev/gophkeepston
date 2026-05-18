package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gophkeeper",
	Short: "GophKeeper password manager",
}

var registerCmd = &cobra.Command{
	Use: "register",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Register command (to be implemented)")
		return nil
	},
}

var loginCmd = &cobra.Command{
	Use: "login",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Login command (to be implemented)")
		return nil
	},
}

func main() {
	rootCmd.AddCommand(registerCmd, loginCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
