// cli/cmd/status.go
package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état du projet",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire")
		}

		config, err := project.LoadConfig(dir)
		if err != nil {
			return err
		}

		fmt.Printf("Projet : %s (v%s)\n", config.Name, config.Version)
		fmt.Printf("Engine : %s\n", config.Engine)
		fmt.Println()

		// Check local services
		fmt.Println("Services locaux :")
		services := []struct {
			name string
			url  string
		}{
			{"Gateway", "http://localhost:8080/health"},
			{"Engine", "http://localhost:4000/health"},
			{"Auth", "http://localhost:4001/health"},
			{"Storage", "http://localhost:4002/health"},
		}

		client := &http.Client{Timeout: 2 * time.Second}
		for _, svc := range services {
			status := "✗ arrêté"
			resp, err := client.Get(svc.url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					status = "✓ en ligne"
				}
			}
			fmt.Printf("  %-12s %s\n", svc.name, status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
