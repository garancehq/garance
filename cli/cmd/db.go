// cli/cmd/db.go
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/garancehq/garance/cli/internal/db"
	"github.com/garancehq/garance/cli/internal/schema"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Commandes de gestion de la base de données",
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Génère et applique une migration depuis garance.schema.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		yes, _ := cmd.Flags().GetBool("yes")

		engineURL := os.Getenv("ENGINE_URL")
		if engineURL == "" {
			engineURL = "http://localhost:4000"
		}

		// Step 1: Compile schema
		fmt.Println("⚙ Compilation du schéma...")
		schemaJSON, err := schema.Compile(dir)
		if err != nil {
			return fmt.Errorf("compilation failed: %w", err)
		}
		fmt.Println("✓ Schéma compilé → garance.schema.json")

		// Step 2: Preview migration
		fmt.Println("\n⚙ Calcul du diff...")
		previewBody, _ := json.Marshal(map[string]json.RawMessage{
			"desired_schema": schemaJSON,
		})

		previewResp, err := http.Post(engineURL+"/api/v1/_migrate/preview", "application/json", bytes.NewReader(previewBody))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine at %s: %w", engineURL, err)
		}
		defer previewResp.Body.Close()

		respBody, _ := io.ReadAll(previewResp.Body)
		if previewResp.StatusCode != 200 {
			return fmt.Errorf("preview failed (%d): %s", previewResp.StatusCode, string(respBody))
		}

		var preview struct {
			SQL          string `json:"sql"`
			Destructive  bool   `json:"destructive"`
			HasChanges   bool   `json:"has_changes"`
			Operations   []struct {
				Op     string `json:"op"`
				Target string `json:"target"`
				Detail string `json:"detail,omitempty"`
			} `json:"operations"`
		}
		if err := json.Unmarshal(respBody, &preview); err != nil {
			return fmt.Errorf("failed to parse preview response: %w", err)
		}

		if !preview.HasChanges {
			fmt.Println("✓ Aucun changement détecté — la base est à jour.")
			return nil
		}

		// Display operations
		fmt.Println("\nOpérations :")
		for _, op := range preview.Operations {
			prefix := "  "
			switch op.Op {
			case "create_table":
				prefix = "  + "
			case "drop_table":
				prefix = "  - "
			case "add_column":
				prefix = "  + "
			case "drop_column":
				prefix = "  - "
			case "alter_column":
				prefix = "  ~ "
			case "rename_column":
				prefix = "  → "
			}
			detail := ""
			if op.Detail != "" {
				detail = " (" + op.Detail + ")"
			}
			fmt.Printf("%s%s %s%s\n", prefix, op.Op, op.Target, detail)
		}

		fmt.Println("\nSQL :")
		fmt.Println(preview.SQL)

		// Step 3: Confirm if destructive
		if preview.Destructive && !yes {
			fmt.Print("\n⚠ Migration destructive détectée. Continuer ? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Migration annulée.")
				return nil
			}
		}

		// Step 4: Save migration file
		migrationsDir := filepath.Join(dir, "migrations")
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}

		timestamp := time.Now().Format("20060102150405")
		description := generateDescription(preview.Operations)
		filename := fmt.Sprintf("%s_%s.sql", timestamp, description)
		migrationPath := filepath.Join(migrationsDir, filename)

		if err := os.WriteFile(migrationPath, []byte(preview.SQL), 0644); err != nil {
			return fmt.Errorf("failed to write migration file: %w", err)
		}
		fmt.Printf("\n✓ Migration sauvegardée → migrations/%s\n", filename)

		// Step 5: Apply migration
		fmt.Println("\n⚙ Application de la migration...")
		applyBody, _ := json.Marshal(map[string]string{
			"sql": preview.SQL,
		})

		applyResp, err := http.Post(engineURL+"/api/v1/_migrate/apply", "application/json", bytes.NewReader(applyBody))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine: %w", err)
		}
		defer applyResp.Body.Close()

		applyRespBody, _ := io.ReadAll(applyResp.Body)
		if applyResp.StatusCode != 200 {
			return fmt.Errorf("apply failed (%d): %s", applyResp.StatusCode, string(applyRespBody))
		}

		fmt.Println("✓ Migration appliquée avec succès")
		return nil
	},
}

var dbResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Supprime et recrée la base de données",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		if err := db.Reset(databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Base de données recréée")

		// Re-apply migrations
		fmt.Println("Application des migrations...")
		if err := db.Migrate(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Migrations appliquées")
		return nil
	},
}

var dbSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Exécute le fichier de seed",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		fmt.Println("Exécution du seed...")
		if err := db.Seed(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Seed exécuté")
		return nil
	},
}

func generateDescription(operations []struct {
	Op     string `json:"op"`
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
}) string {
	if len(operations) == 0 {
		return "migration"
	}
	if len(operations) == 1 {
		op := operations[0]
		return strings.ReplaceAll(op.Op+"_"+op.Target, ".", "_")
	}
	// Summarize: count unique op types
	ops := make(map[string]int)
	for _, op := range operations {
		ops[op.Op]++
	}
	parts := make([]string, 0, len(ops))
	for op, count := range ops {
		parts = append(parts, fmt.Sprintf("%s_%d", op, count))
	}
	return strings.Join(parts, "_")
}

func init() {
	dbMigrateCmd.Flags().Bool("yes", false, "Applique automatiquement sans confirmation")
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbResetCmd)
	dbCmd.AddCommand(dbSeedCmd)
	rootCmd.AddCommand(dbCmd)
}
