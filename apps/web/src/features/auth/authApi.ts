import { apiClient } from '../../api/client'

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

export async function register(input: RegisterInput): Promise<TokenPair> {
  const res = await apiClient.post<{ data: TokenPair }>('/v1/auth/register', input)
  return res.data.data
}

export async function login(input: LoginInput): Promise<TokenPair> {
  const res = await apiClient.post<{ data: TokenPair }>('/v1/auth/login', input)
  return res.data.data
}
