/** Root schema -- output of compile() and content of garance.schema.json */
export interface GaranceSchema {
  version: number
  tables: Record<string, GaranceTable>
  storage: Record<string, GaranceBucket>
}

export interface GaranceTable {
  columns: Record<string, GaranceColumn>
  relations: Record<string, GaranceRelation>
  access?: GaranceAccess
}

export interface GaranceColumn {
  type: string
  primary_key: boolean
  unique: boolean
  nullable: boolean
  default?: string
  references?: string
  renamed_from?: string
}

export interface GaranceRelation {
  type: 'hasMany' | 'hasOne' | 'belongsTo'
  table: string
  foreign_key: string
}

export interface GaranceAccess {
  read?: AccessRule
  write?: AccessRule
  delete?: AccessRule
}

export type AccessRule = string | AccessCondition[]

export interface AccessCondition {
  type: 'isOwner' | 'isAuthenticated' | 'where'
  column?: string
  filters?: Record<string, unknown>
}

export interface GaranceBucket {
  max_file_size?: string
  allowed_mime_types?: string[]
  access?: GaranceAccess
}
