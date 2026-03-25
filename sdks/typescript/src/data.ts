import type { HttpClient } from './client'

export class QueryBuilder<T = unknown> {
  private _table: string
  private _http: HttpClient
  private _params: Record<string, string> = {}
  private _selectCols?: string

  constructor(http: HttpClient, table: string) {
    this._http = http
    this._table = table
  }

  select(columns: string) {
    this._selectCols = columns
    return this
  }

  eq(column: string, value: string | number | boolean) {
    this._params[column] = `eq.${value}`
    return this
  }

  neq(column: string, value: string | number | boolean) {
    this._params[column] = `neq.${value}`
    return this
  }

  gt(column: string, value: string | number) {
    this._params[column] = `gt.${value}`
    return this
  }

  gte(column: string, value: string | number) {
    this._params[column] = `gte.${value}`
    return this
  }

  lt(column: string, value: string | number) {
    this._params[column] = `lt.${value}`
    return this
  }

  lte(column: string, value: string | number) {
    this._params[column] = `lte.${value}`
    return this
  }

  order(column: string, direction: 'asc' | 'desc' = 'asc') {
    this._params['order'] = `${column}.${direction}`
    return this
  }

  limit(n: number) {
    this._params['limit'] = String(n)
    return this
  }

  offset(n: number) {
    this._params['offset'] = String(n)
    return this
  }

  /** Execute the query — returns array of rows */
  async execute() {
    const params = { ...this._params }
    if (this._selectCols) params['select'] = this._selectCols
    return this._http.get<T[]>(`/api/v1/${this._table}`, params)
  }

  /** Get a single row by ID */
  async getById(id: string) {
    return this._http.get<T>(`/api/v1/${this._table}/${id}`)
  }

  /** Insert a new row */
  async insert(data: Partial<T>) {
    return this._http.post<T>(`/api/v1/${this._table}`, data)
  }

  /** Update a row by ID */
  async update(id: string, data: Partial<T>) {
    return this._http.patch<T>(`/api/v1/${this._table}/${id}`, data)
  }

  /** Delete a row by ID */
  async deleteById(id: string) {
    return this._http.delete(`/api/v1/${this._table}/${id}`)
  }
}
