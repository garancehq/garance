// cli/cmd/dev.go
package cmd

import (
	"fmt"
	"os"

	"github.com/garancehq/garance/cli/internal/compose"
	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Lance l'environnement de développement local",
	Long:  "Démarre PostgreSQL, MinIO, Engine, Auth, Storage et Gateway via Docker Compose",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire (lancez 'garance init' d'abord)")
		}

		fmt.Println("Démarrage de l'environnement Garance...")
		fmt.Println()

		if err := compose.Up(dir); err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("✓ Environnement démarré")
		fmt.Println()
		fmt.Println("Services disponibles :")
		fmt.Println("  Gateway (API)      http://localhost:8080")
		fmt.Println("  Engine             http://localhost:4000")
		fmt.Println("  Auth               http://localhost:4001")
		fmt.Println("  Storage            http://localhost:4002")
		fmt.Println("  MinIO Console      http://localhost:9001")
		fmt.Println("  PostgreSQL         localhost:5432")
		fmt.Println()
		fmt.Println("Commandes utiles :")
		fmt.Println("  garance dev stop   — arrête l'environnement")
		fmt.Println("  garance dev status — état des services")
		fmt.Println("  garance dev logs   — affiche les logs")

		return nil
	},
}

var devStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Arrête l'environnement de développement",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		if err := compose.Down(dir); err != nil {
			return err
		}
		fmt.Println("✓ Environnement arrêté")
		return nil
	},
}

var devStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état des services",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		return compose.Status(dir)
	},
}

var devLogsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Affiche les logs des services",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		service := ""
		if len(args) > 0 {
			service = args[0]
		}
		follow, _ := cmd.Flags().GetBool("follow")
		return compose.Logs(dir, service, follow)
	},
}

func init() {
	devLogsCmd.Flags().BoolP("follow", "f", false, "Suivre les logs en temps réel")
	devCmd.AddCommand(devStopCmd)
	devCmd.AddCommand(devStatusCmd)
	devCmd.AddCommand(devLogsCmd)
	rootCmd.AddCommand(devCmd)
}
