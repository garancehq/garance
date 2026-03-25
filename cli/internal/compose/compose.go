// cli/internal/compose/compose.go
package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const composeFileName = ".garance-compose.yml"

// WriteComposeFile writes the dev Docker Compose file to the project directory.
func WriteComposeFile(dir string) (string, error) {
	path := filepath.Join(dir, composeFileName)
	content := DevComposeTemplate()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write compose file: %w", err)
	}
	return path, nil
}

// Up starts the dev environment.
func Up(dir string) error {
	composePath, err := WriteComposeFile(dir)
	if err != nil {
		return err
	}

	// Load .env.local if it exists
	envFile := filepath.Join(dir, ".env.local")
	args := []string{"compose", "-f", composePath, "--project-name", "garance"}
	if _, err := os.Stat(envFile); err == nil {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "up", "-d")

	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

// Down stops the dev environment.
func Down(dir string) error {
	composePath := filepath.Join(dir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("no dev environment found (run 'garance dev' first)")
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "--project-name", "garance", "down")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

// Status shows running services.
func Status(dir string) error {
	composePath := filepath.Join(dir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("no dev environment found (run 'garance dev' first)")
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "--project-name", "garance", "ps")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Logs shows service logs.
func Logs(dir string, service string, follow bool) error {
	composePath := filepath.Join(dir, composeFileName)
	args := []string{"compose", "-f", composePath, "--project-name", "garance", "logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
