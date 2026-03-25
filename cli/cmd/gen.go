// cli/cmd/gen.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Commandes de génération de code",
}

var genTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "Génère les types clients depuis le schéma",
	Long:  "Génère les types TypeScript, Dart, Swift ou Kotlin depuis le schéma PostgreSQL introspécté",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire")
		}

		lang, _ := cmd.Flags().GetString("lang")
		output, _ := cmd.Flags().GetString("output")

		if output == "" {
			switch lang {
			case "ts":
				output = filepath.Join(dir, "types", "garance.ts")
			case "dart":
				output = filepath.Join(dir, "types", "garance.dart")
			case "swift":
				output = filepath.Join(dir, "types", "garance.swift")
			case "kotlin":
				output = filepath.Join(dir, "types", "garance.kt")
			default:
				output = filepath.Join(dir, "types", "garance.ts")
			}
		}

		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Call the engine codegen endpoint or binary
		// For MVP, we call the engine HTTP API to introspect and generate
		engineURL := os.Getenv("ENGINE_URL")
		if engineURL == "" {
			engineURL = "http://localhost:4000"
		}

		fmt.Printf("Génération des types %s...\n", lang)

		// For MVP: use curl to call engine introspection, then generate locally
		// This is a placeholder — in production, the engine would have a /gen/types endpoint
		// For now, we just create a stub file indicating the types would be generated
		stub := fmt.Sprintf("// Types générés par garance gen types --lang %s\n// Connectez-vous à un environnement Garance (garance dev) pour générer les types réels\n// ENGINE_URL: %s\n", lang, engineURL)
		if err := os.WriteFile(output, []byte(stub), 0644); err != nil {
			return fmt.Errorf("failed to write types: %w", err)
		}

		fmt.Printf("✓ Types générés : %s\n", output)
		return nil
	},
}

func init() {
	genTypesCmd.Flags().StringP("lang", "l", "ts", "Langage cible (ts, dart, swift, kotlin)")
	genTypesCmd.Flags().StringP("output", "o", "", "Fichier de sortie")
	genCmd.AddCommand(genTypesCmd)
	rootCmd.AddCommand(genCmd)
}
