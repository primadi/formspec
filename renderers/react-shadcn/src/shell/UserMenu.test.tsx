// @vitest-environment jsdom
//
// UserMenu — the signed-in identity control, and the CONTEXT SWITCHER it hosts
// (kafe 6.5.10 / plan session-context-role-branch.md tahap 4).
//
// Why the switcher lives here rather than on a route: the whole promise of
// "one session, one (role × branch)" is that the boundary can be CHANGED
// deliberately — the picker at login makes it true once, and the switcher makes
// it true for the rest of the session. Without it a cashier moving between
// outlets has to sign out and back in.

import { describe, it, expect, beforeEach } from "vitest"
import { render, screen, cleanup, fireEvent } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import { UserMenu } from "./UserMenu"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import type { ContextChoice, MeResponse, MetaBundle } from "@/types/manifest"

function makeBundle(): MetaBundle {
  return {
    app: { name: "kafe-pos", root_url: "/app", chrome: {} },
  } as unknown as MetaBundle
}

function makeMe(over: Partial<MeResponse> = {}): MeResponse {
  return {
    user_id: "u-1",
    username: "kasir2",
    workspace: "kafe",
    app: "kafe-pos",
    roles: ["sales"],
    permissions: [],
    ...over,
  }
}

const choices: ContextChoice[] = [
  {
    id: "sales@KFE-JKT-01",
    role: "sales",
    dimension: "branch_id",
    value: "KFE-JKT-01",
  },
  {
    id: "admin@KFE-BDG-01",
    role: "admin",
    dimension: "branch_id",
    value: "KFE-BDG-01",
  },
]

function renderMenu() {
  return render(
    <MemoryRouter initialEntries={["/kafe/app/"]}>
      <Routes>
        <Route path="/:workspace/*" element={<UserMenu />} />
      </Routes>
    </MemoryRouter>,
  )
}

/** Open the dropdown (base-ui opens on click, not hover). */
function openMenu() {
  fireEvent.click(screen.getByLabelText("User menu"))
}

beforeEach(() => {
  cleanup()
  useMetaStore.setState({ bundle: makeBundle() })
  useSessionStore.setState({
    token: "tok",
    workspace: "kafe",
    app: "kafe-pos",
    refreshToken: "refresh",
    me: makeMe(),
  })
})

describe("UserMenu — active context", () => {
  it("shows the context the session is acting in, not just the principal's roles", () => {
    useSessionStore.setState({
      me: makeMe({
        context: { role: "sales", dimension: "branch_id", value: "KFE-JKT-01" },
        context_choices: choices,
      }),
    })
    renderMenu()
    openMenu()

    // The audit question is "as which role, in which branch" — `roles: [sales]`
    // alone cannot answer it, so the menu shows the boundary itself.
    expect(screen.getByTestId("active-context")).toHaveTextContent(
      "sales · KFE-JKT-01",
    )
  })

  it("offers no switcher when there is nothing to switch to", () => {
    useSessionStore.setState({
      me: makeMe({
        context: { role: "owner" },
        context_choices: [
          { id: "owner@", role: "owner", dimension: "", value: "" },
        ],
      }),
    })
    renderMenu()
    openMenu()

    // A boundary-less owner has one (or zero) contexts; a menu item that can
    // never do anything is noise, so it is absent rather than disabled.
    expect(screen.queryByText("Switch context")).not.toBeInTheDocument()
  })

  it("offers the switcher when the principal holds several contexts", () => {
    useSessionStore.setState({
      me: makeMe({
        context: { role: "sales", dimension: "branch_id", value: "KFE-JKT-01" },
        context_choices: choices,
      }),
    })
    renderMenu()
    openMenu()

    expect(screen.getByText("Switch context")).toBeInTheDocument()
  })

  it("opens the picker prefilled from the remembered choice", () => {
    localStorage.setItem("formspec-context:kafe:kafe-pos", "admin@KFE-BDG-01")
    useSessionStore.setState({
      me: makeMe({
        context: { role: "sales", dimension: "branch_id", value: "KFE-JKT-01" },
        context_choices: choices,
      }),
    })
    renderMenu()
    openMenu()
    fireEvent.click(screen.getByText("Switch context"))

    const radios = screen.getAllByRole("radio") as HTMLInputElement[]
    const checked = radios.filter((r) => r.checked)
    // Prefilled, never auto-submitted: the boundary stays something the caller
    // states, which is what keeps the audit answer true.
    expect(checked).toHaveLength(1)
    expect(checked[0].value).toBe("admin@KFE-BDG-01")
    localStorage.clear()
  })
})
