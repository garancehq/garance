package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialise un nouveau projet Garance",
	Long:  "Crée la structure de fichiers pour un nouveau projet Garance (garance.json, garance.schema.ts, etc.)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		name := filepath.Base(dir)
		if len(args) > 0 {
			name = args[0]
			// Create subdirectory if name is provided
			dir = filepath.Join(dir, name)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if project.Exists(dir) {
			return fmt.Errorf("project already initialized (garance.json exists)")
		}

		if err := project.Init(dir, name); err != nil {
			return err
		}

		fmt.Printf("✓ Projet '%s' initialisé\n", name)
		fmt.Println()
		fmt.Println("Fichiers créés :")
		fmt.Println("  garance.json        — configuration du projet")
		fmt.Println("  garance.schema.ts   — schéma déclaratif")
		fmt.Println("  seeds/seed.sql      — données de test")
		fmt.Println("  migrations/         — migrations SQL")
		fmt.Println("  .env.local          — variables d'environnement locales")
		fmt.Println()
		fmt.Println("Prochaine étape :")
		fmt.Println("  garance dev         — lance l'environnement de développement")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
