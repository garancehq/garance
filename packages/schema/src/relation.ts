import type { GaranceRelation } from './types'

class RelationBuilder {
  private _type: GaranceRelation['type']
  private _table: string
  private _foreignKey: string

  constructor(type: GaranceRelation['type'], table: string, foreignKey: string) {
    this._type = type
    this._table = table
    this._foreignKey = foreignKey
  }

  /** @internal */
  _build(): GaranceRelation {
    return {
      type: this._type,
      table: this._table,
      foreign_key: this._foreignKey,
    }
  }
}

export const relation = {
  hasMany: (table: string, foreignKey: string) => new RelationBuilder('hasMany', table, foreignKey),
  hasOne: (table: string, foreignKey: string) => new RelationBuilder('hasOne', table, foreignKey),
  belongsTo: (table: string, foreignKey: string) => new RelationBuilder('belongsTo', table, foreignKey),
}

export { RelationBuilder }
