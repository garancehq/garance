// cli/cmd/db.go
package cmd

import (
	"fmt"
	"os"

	"github.com/garancehq/garance/cli/internal/db"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Commandes de gestion de la base de données",
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Applique les migrations SQL",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		fmt.Println("Application des migrations...")
		if err := db.Migrate(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Migrations appliquées")
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

func init() {
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbResetCmd)
	dbCmd.AddCommand(dbSeedCmd)
	rootCmd.AddCommand(dbCmd)
}
