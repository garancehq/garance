import type { GaranceTable, GaranceAccess, AccessRule } from './types'
import type { ColumnBuilder } from './column'
import type { RelationBuilder } from './relation'

interface AccessContext {
  isOwner(column: string): AccessCondition
  isAuthenticated(): AccessCondition
  where(filters: Record<string, unknown>): AccessConditionBuilder
}

interface AccessCondition {
  type: 'isOwner' | 'isAuthenticated' | 'where'
  column?: string
  filters?: Record<string, unknown>
}

class AccessConditionBuilder {
  private conditions: AccessCondition[] = []

  constructor(condition: AccessCondition) {
    this.conditions.push(condition)
  }

  or(other: AccessCondition | AccessConditionBuilder): AccessConditionBuilder {
    if (other instanceof AccessConditionBuilder) {
      this.conditions.push(...other.conditions)
    } else {
      this.conditions.push(other)
    }
    return this
  }

  /** @internal */
  _build(): AccessCondition[] {
    return this.conditions
  }
}

const accessContext: AccessContext = {
  isOwner: (column: string) => ({ type: 'isOwner', column }),
  isAuthenticated: () => ({ type: 'isAuthenticated' }),
  where: (filters: Record<string, unknown>) => new AccessConditionBuilder({ type: 'where', filters }),
}

type AccessRuleFn = (ctx: AccessContext) => AccessCondition | AccessConditionBuilder

interface TableAccess {
  read?: AccessRuleFn | 'public'
  write?: AccessRuleFn
  delete?: AccessRuleFn
}

interface TableDefinition {
  [key: string]: ColumnBuilder | RelationBuilder | TableAccess
}

function isColumnBuilder(v: unknown): v is ColumnBuilder {
  return v !== null && typeof v === 'object' && '_build' in v && '_type' in (v as Record<string, unknown>)
}

function isRelationBuilder(v: unknown): v is RelationBuilder {
  return v !== null && typeof v === 'object' && '_build' in v && '_table' in (v as Record<string, unknown>)
}

class TableBuilder {
  private definition: TableDefinition

  constructor(definition: TableDefinition) {
    this.definition = definition
  }

  /** @internal */
  _build(): GaranceTable {
    const columns: GaranceTable['columns'] = {}
    const relations: GaranceTable['relations'] = {}
    let access: GaranceAccess | undefined

    for (const [key, value] of Object.entries(this.definition)) {
      if (key === 'access') {
        access = buildAccess(value as TableAccess)
      } else if (isRelationBuilder(value)) {
        relations[key] = (value as RelationBuilder)._build()
      } else if (isColumnBuilder(value)) {
        columns[key] = (value as ColumnBuilder)._build()
      }
    }

    return { columns, relations, ...(access && { access }) }
  }
}

function buildAccess(def: TableAccess): GaranceAccess {
  const result: GaranceAccess = {}

  if (def.read !== undefined) {
    result.read = typeof def.read === 'string' ? def.read : buildAccessRule(def.read)
  }
  if (def.write !== undefined) {
    result.write = buildAccessRule(def.write)
  }
  if (def.delete !== undefined) {
    result.delete = buildAccessRule(def.delete)
  }

  return result
}

function buildAccessRule(fn: AccessRuleFn): AccessRule {
  const result = fn(accessContext)
  if (result instanceof AccessConditionBuilder) {
    return result._build()
  }
  return [result]
}

export function table(definition: TableDefinition): TableBuilder {
  return new TableBuilder(definition)
}

export { TableBuilder }
