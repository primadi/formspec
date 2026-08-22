// ─── Auth API ───
//
// Username/password login against the external surface (/api/v1/auth/login).
// Returns the JWT access token used to authenticate the UI session.

import ky, { HTTPError } from "ky"

export interface LoginResult {
  accessToken: string
  refreshToken: string
}

/**
 * Log in with username/password and return the JWT token pair.
 * Throws an Error with the server's message on invalid credentials.
 */
export async function loginWithPassword(
  workspace: string,
  username: string,
  password: string,
): Promise<LoginResult> {
  try {
    const response = await ky.post(`/${workspace}/api/v1/auth/login`, {
      json: { username, password },
      retry: 0,
    })
    const body = (await response.json()) as {
      data: { access_token: string; refresh_token: string }
    }
    return {
      accessToken: body.data.access_token,
      refreshToken: body.data.refresh_token,
    }
  } catch (err) {
    if (err instanceof HTTPError) {
      const body = (await err.response
        .clone()
        .json()
        .catch(() => ({}))) as {
        error?: { message?: string }
      }
      throw new Error(body.error?.message ?? "Login failed")
    }
    throw err
  }
}
