package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Compile(dir string) (json.RawMessage, error) {
	schemaFile := filepath.Join(dir, "garance.schema.ts")
	if _, err := os.Stat(schemaFile); err != nil {
		return nil, fmt.Errorf("garance.schema.ts not found in %s", dir)
	}

	tmpScript := filepath.Join(dir, ".garance-compile.mjs")
	script := `import { compile } from '@garance/schema';
const mod = await import('./garance.schema.ts');
const schema = mod.default;
const result = compile(schema);
console.log(JSON.stringify(result));
`
	if err := os.WriteFile(tmpScript, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("failed to write compile script: %w", err)
	}
	defer os.Remove(tmpScript)

	cmd := exec.Command("npx", "tsx", tmpScript)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("schema compilation failed: %w\nMake sure Node.js is installed and run 'npm install' in your project", err)
	}

	var raw json.RawMessage
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("schema compilation produced invalid JSON: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "garance.schema.json"), output, 0644); err != nil {
		return nil, fmt.Errorf("failed to write garance.schema.json: %w", err)
	}

	return raw, nil
}
