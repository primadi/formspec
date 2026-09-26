import { create } from "zustand"

interface RouteIdentityState {
  identities: Record<string, string>
  setIdentity: (pathname: string, identifier: string) => void
  getIdentity: (pathname: string) => string | undefined
}

export const useRouteIdentityStore = create<RouteIdentityState>((set, get) => ({
  identities: {},
  setIdentity: (pathname, identifier) =>
    set((state) => ({
      identities: { ...state.identities, [pathname]: identifier },
    })),
  getIdentity: (pathname) => get().identities[pathname],
}))
