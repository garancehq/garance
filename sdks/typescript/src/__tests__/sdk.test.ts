import { describe, it, expect } from 'vitest'
import { createClient } from '../index'

describe('createClient', () => {
  it('creates a client with auth, from, and storage namespaces', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    expect(garance.auth).toBeDefined()
    expect(garance.from).toBeInstanceOf(Function)
    expect(garance.storage).toBeDefined()
  })
})

describe('QueryBuilder', () => {
  it('builds a query with filters', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const query = garance.from('users').eq('name', 'Alice').limit(10)
    expect(query).toBeDefined()
  })
})

describe('StorageClient', () => {
  it('generates public URL', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const url = garance.storage.from('avatars').getPublicUrl('user-123/photo.jpg')
    expect(url).toBe('http://localhost:8080/storage/v1/avatars/user-123/photo.jpg')
  })

  it('generates public URL with transforms', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const url = garance.storage.from('avatars').getPublicUrl('photo.jpg', {
      transform: { width: 200, height: 200, format: 'webp' },
    })
    expect(url).toContain('w=200')
    expect(url).toContain('h=200')
    expect(url).toContain('f=webp')
  })
})

describe('SDK matches spec examples', () => {
  it('has the same API shape as documented', () => {
    const garance = createClient({ url: 'https://mon-projet.garance.io' })

    // These should all be valid calls (we can't test network, but verify the API exists)
    expect(typeof garance.auth.signUp).toBe('function')
    expect(typeof garance.auth.signIn).toBe('function')
    expect(typeof garance.auth.signInWithMagicLink).toBe('function')
    expect(typeof garance.auth.signInWithOAuth).toBe('function')
    expect(typeof garance.auth.getUser).toBe('function')
    expect(typeof garance.auth.signOut).toBe('function')

    const query = garance.from('users')
    expect(typeof query.execute).toBe('function')
    expect(typeof query.insert).toBe('function')
    expect(typeof query.getById).toBe('function')

    const storage = garance.storage.from('avatars')
    expect(typeof storage.upload).toBe('function')
    expect(typeof storage.getPublicUrl).toBe('function')
    expect(typeof storage.createSignedUrl).toBe('function')
  })
})
