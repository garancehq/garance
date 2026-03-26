import type { GaranceColumn } from './types'

class ColumnBuilder {
  private _type: string
  private _primaryKey = false
  private _unique = false
  private _nullable = true // nullable by default in PG
  private _default?: string
  private _references?: string
  private _renamedFrom?: string

  constructor(type: string) {
    this._type = type
  }

  primaryKey(): this {
    this._primaryKey = true
    this._nullable = false
    return this
  }

  unique(): this {
    this._unique = true
    return this
  }

  notNull(): this {
    this._nullable = false
    return this
  }

  nullable(): this {
    this._nullable = true
    return this
  }

  default(value: string): this {
    this._default = value
    return this
  }

  references(ref: string): this {
    this._references = ref
    return this
  }

  renamedFrom(oldName: string): this {
    this._renamedFrom = oldName
    return this
  }

  /** @internal */
  _build(): GaranceColumn {
    return {
      type: this._type,
      primary_key: this._primaryKey,
      unique: this._unique,
      nullable: this._nullable,
      ...(this._default !== undefined && { default: this._default }),
      ...(this._references !== undefined && { references: this._references }),
      ...(this._renamedFrom && { renamed_from: this._renamedFrom }),
    }
  }
}

export const column = {
  uuid: () => new ColumnBuilder('uuid'),
  text: () => new ColumnBuilder('text'),
  int4: () => new ColumnBuilder('int4'),
  int8: () => new ColumnBuilder('int8'),
  float8: () => new ColumnBuilder('float8'),
  bool: () => new ColumnBuilder('bool'),
  boolean: () => new ColumnBuilder('bool'),
  timestamp: () => new ColumnBuilder('timestamp'),
  timestamptz: () => new ColumnBuilder('timestamptz'),
  date: () => new ColumnBuilder('date'),
  jsonb: () => new ColumnBuilder('jsonb'),
  json: () => new ColumnBuilder('json'),
  bytea: () => new ColumnBuilder('bytea'),
  numeric: () => new ColumnBuilder('numeric'),
  serial: () => new ColumnBuilder('serial'),
  bigserial: () => new ColumnBuilder('bigserial'),
}

export { ColumnBuilder }
