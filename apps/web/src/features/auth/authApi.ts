import { getAuth } from '@/api/generated/auth/auth'

export interface TokenPair {
  access_token: string
  refresh_token: string
}

export interface RegisterInput {
  email: string
  password: string
}

export interface LoginInput {
  email: string
  password: string
}

const api = getAuth()

export async function register(input: RegisterInput): Promise<TokenPair> {
  const env = await api.postV1AuthRegister({ email: input.email, password: input.password })
  return env.data as TokenPair
}

export async function login(input: LoginInput): Promise<TokenPair> {
  const env = await api.postV1AuthLogin({ email: input.email, password: input.password })
  return env.data as TokenPair
}

export async function refreshTokens(refreshToken: string): Promise<TokenPair> {
  const env = await api.postV1AuthRefresh({ refresh_token: refreshToken })
  return env.data as TokenPair
}
