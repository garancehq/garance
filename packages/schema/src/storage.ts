import type { GaranceBucket, GaranceAccess, AccessCondition } from './types'

interface BucketAccessContext {
  isAuthenticated(): AccessCondition
  isOwner(): AccessCondition
}

interface BucketDefinition {
  maxFileSize?: string
  allowedMimeTypes?: string[]
  access?: {
    read?: 'public' | ((ctx: BucketAccessContext) => AccessCondition)
    write?: (ctx: BucketAccessContext) => AccessCondition
    delete?: (ctx: BucketAccessContext) => AccessCondition
  }
}

class BucketBuilder {
  private definition: BucketDefinition

  constructor(definition: BucketDefinition) {
    this.definition = definition
  }

  /** @internal */
  _build(): GaranceBucket {
    const result: GaranceBucket = {}

    if (this.definition.maxFileSize) {
      result.max_file_size = this.definition.maxFileSize
    }
    if (this.definition.allowedMimeTypes) {
      result.allowed_mime_types = this.definition.allowedMimeTypes
    }
    if (this.definition.access) {
      const access: GaranceAccess = {}
      const ctx: BucketAccessContext = {
        isAuthenticated: () => ({ type: 'isAuthenticated' }),
        isOwner: () => ({ type: 'isOwner' }),
      }

      if (this.definition.access.read !== undefined) {
        access.read = typeof this.definition.access.read === 'string'
          ? this.definition.access.read
          : [this.definition.access.read(ctx)]
      }
      if (this.definition.access.write) {
        access.write = [this.definition.access.write(ctx)]
      }
      if (this.definition.access.delete) {
        access.delete = [this.definition.access.delete(ctx)]
      }
      result.access = access
    }

    return result
  }
}

export function bucket(definition: BucketDefinition): BucketBuilder {
  return new BucketBuilder(definition)
}

export { BucketBuilder }
