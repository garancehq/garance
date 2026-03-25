package project

func DefaultSchemaTemplate() string {
	return `import { defineSchema, table, column, relation } from '@garance/schema'

export default defineSchema({
  // Example table — customize or replace with your own schema
  users: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    email: column.text().unique().notNull(),
    name: column.text().notNull(),
    created_at: column.timestamp().default('now()'),
  }),
})
`
}

func DefaultSeedTemplate() string {
	return `-- Seed data for local development
-- Run with: garance db seed

-- INSERT INTO users (email, name) VALUES ('dev@example.fr', 'Dev User');
`
}

func DefaultEnvTemplate() string {
	return `# Garance local development environment
# These values are used by 'garance dev'

DATABASE_URL=postgresql://postgres:postgres@localhost:5432/garance
JWT_SECRET=dev-secret-change-me-in-production
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
`
}
